package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"
)

//go:embed Dockerfile
var dockerfile string

type options struct {
	project, workflow, job, event, eventFile, arch, out, secretFile, varFile string
	plan, list, dockerSocket                                                 bool
	platforms                                                                mappings
	timeout                                                                  time.Duration
	vm                                                                       string
	cacheDir                                                                 string
	nixCache                                                                 bool
	runID                                                                    string
}

type mappings map[string]string

func (m mappings) String() string { return fmt.Sprint(map[string]string(m)) }
func (m mappings) Set(v string) error {
	k, value, ok := strings.Cut(v, "=")
	if !ok || k == "" || value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(v, "\n\r") {
		return errors.New("platform must be LABEL=DOCKER_IMAGE")
	}
	m[k] = value
	return nil
}

type job struct {
	RunsOn    yaml.Node `yaml:"runs-on"`
	Needs     yaml.Node `yaml:"needs"`
	Uses      string    `yaml:"uses"`
	Container yaml.Node `yaml:"container"`
	Strategy  struct {
		Matrix yaml.Node `yaml:"matrix"`
	} `yaml:"strategy"`
}
type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type image struct{ Label, Base, Nix, Tag, ID string }
type executionPlan struct {
	Project           string           `json:"project"`
	Workflow          string           `json:"workflow"`
	Job               string           `json:"job,omitempty"`
	Event             string           `json:"event"`
	Architecture      string           `json:"architecture"`
	Images            []image          `json:"images"`
	Warnings          []string         `json:"warnings"`
	VM                string           `json:"vm,omitempty"`
	CacheDir          string           `json:"cache_dir,omitempty"`
	NixCache          *nixCacheVolumes `json:"nix_cache,omitempty"`
	NixCacheRequested bool             `json:"nix_cache_requested"`
}
type result struct {
	Plan              executionPlan      `json:"plan"`
	Started           string             `json:"started"`
	Finished          string             `json:"finished"`
	Status            string             `json:"status"`
	Phase             string             `json:"phase"`
	Error             string             `json:"error,omitempty"`
	Excluded          []string           `json:"excluded,omitempty"`
	ActVersion        string             `json:"act_version,omitempty"`
	Jobs              []jobOutcome       `json:"jobs,omitempty"`
	DockerPlatform    string             `json:"docker_platform,omitempty"`
	ElapsedSeconds    map[string]float64 `json:"elapsed_seconds"`
	RunLabel          string             `json:"run_label"`
	StoppedContainers []string           `json:"stopped_containers,omitempty"`
}

type jobOutcome struct {
	ID     string         `json:"id"`
	Matrix map[string]any `json:"matrix,omitempty"`
	Result string         `json:"result"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func parse(args []string) (options, error) {
	o := options{platforms: mappings{}}
	f := flag.NewFlagSet("ci-self sandbox", flag.ContinueOnError)
	f.StringVar(&o.project, "project-dir", ".", "CI target project path (Git checkout)")
	f.StringVar(&o.workflow, "workflow", "", "workflow path relative to project; required when multiple exist")
	f.StringVar(&o.job, "job", "", "job ID (and its dependencies); default all jobs")
	f.StringVar(&o.event, "event", "push", "workflow event")
	f.StringVar(&o.eventFile, "event-file", "", "explicit event JSON; default {\"act\":true}")
	f.StringVar(&o.arch, "arch", "linux/amd64", "container architecture: linux/amd64 or linux/arm64")
	f.StringVar(&o.vm, "vm", "", "use Docker inside this local Lima VM, e.g. ci-self-x64")
	f.BoolVar(&o.nixCache, "nix-cache", false, "persist project-specific Nix store and evaluation cache on the Docker engine")
	f.StringVar(&o.cacheDir, "cache-dir", "", "persistent act actions/cache parent (default user cache/ci-self-runner/act)")
	f.StringVar(&o.out, "out", "", "parent for new run directories; default user cache/ci-self-runner/sandbox")
	f.StringVar(&o.secretFile, "secret-file", "", "explicit act secrets file; no automatic host credentials")
	f.StringVar(&o.varFile, "var-file", "", "explicit repository variables file")
	f.BoolVar(&o.plan, "plan", false, "print compatibility plan without creating files or running containers")
	f.BoolVar(&o.list, "list", false, "list jobs via act without running containers")
	f.BoolVar(&o.dockerSocket, "docker-socket", false, "allow workflow access to Docker daemon (host control)")
	f.DurationVar(&o.timeout, "timeout", 45*time.Minute, "overall execution timeout (act does not enforce job timeout-minutes)")
	f.Var(o.platforms, "platform", "explicit LABEL=BASE_IMAGE override; repeatable, Linux containers only")
	f.Usage = func() {
		fmt.Fprintln(f.Output(), "Usage: ci-self sandbox --project-dir PATH [--workflow .github/workflows/ci.yml] [--job ID] [--plan|--list]\nRuns the original workflow with act in Ubuntu + Nix. Does not emulate GitHub services or macOS/Windows.")
		f.PrintDefaults()
	}
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if len(f.Args()) != 0 {
		return o, errors.New("unexpected positional arguments")
	}
	if o.arch != "linux/amd64" && o.arch != "linux/arm64" {
		return o, errors.New("--arch must be linux/amd64 or linux/arm64")
	}
	if o.event == "" || strings.HasPrefix(o.event, "-") {
		return o, errors.New("invalid --event")
	}
	if o.timeout <= 0 {
		return o, errors.New("--timeout must be positive")
	}
	if o.vm != "" && !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`).MatchString(o.vm) {
		return o, errors.New("invalid --vm instance name")
	}
	var err error
	caller := os.Getenv("CI_SELF_CALLER_DIR")
	if caller != "" && !filepath.IsAbs(o.project) {
		o.project = filepath.Join(caller, o.project)
	}
	o.project, err = filepath.Abs(o.project)
	if err != nil {
		return o, err
	}
	o.project, err = filepath.EvalSymlinks(o.project)
	if err != nil {
		return o, fmt.Errorf("project: %w", err)
	}
	for _, p := range []*string{&o.eventFile, &o.secretFile, &o.varFile, &o.out, &o.cacheDir} {
		if *p != "" {
			if caller != "" && !filepath.IsAbs(*p) {
				*p = filepath.Join(caller, *p)
			}
			*p, err = filepath.Abs(*p)
			if err != nil {
				return o, err
			}
		}
	}
	if o.cacheDir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return o, err
		}
		o.cacheDir = filepath.Join(cache, "ci-self-runner", "act")
	}
	return o, nil
}

func workflowPath(project, requested string) (string, error) {
	var err error
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return "", err
	}
	if requested == "" {
		var files []string
		for _, pattern := range []string{"*.yml", "*.yaml"} {
			matches, _ := filepath.Glob(filepath.Join(project, ".github/workflows", pattern))
			files = append(files, matches...)
		}
		sort.Strings(files)
		if len(files) != 1 {
			return "", fmt.Errorf("found %d workflows; specify --workflow (candidates: %s)", len(files), strings.Join(files, ", "))
		}
		requested = files[0]
	}
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(project, requested)
	}
	resolved, err := filepath.EvalSymlinks(requested)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(project, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return "", errors.New("workflow must be inside the project")
	}
	return rel, nil
}

func stringList(n yaml.Node) ([]string, error) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
		return []string{n.Value}, nil
	}
	if n.Kind == yaml.SequenceNode {
		var result []string
		for _, child := range n.Content {
			if child.Kind != yaml.ScalarNode || child.Tag != "!!str" {
				return nil, errors.New("expected string labels")
			}
			result = append(result, child.Value)
		}
		return result, nil
	}
	return nil, errors.New("expected a runner label or label list")
}

func makePlan(o options) (executionPlan, error) {
	p := executionPlan{Project: o.project, Job: o.job, Event: o.event, Architecture: o.arch,
		Warnings: []string{
			"act is not GitHub Actions: OIDC, permissions, environments, concurrency and timeout behavior differ",
			"Ubuntu container + Nix, not a GitHub VM; kernel, services and installed tools may differ",
			"workflow steps are unchanged; Nix dependencies are used only where the workflow invokes Nix",
			"containers run untrusted workflow code; network is enabled; no host credentials are auto-injected",
		}}
	p.VM = o.vm
	p.CacheDir = o.cacheDir
	p.NixCacheRequested = o.nixCache
	var err error
	p.Workflow, err = workflowPath(o.project, o.workflow)
	if err != nil {
		return p, err
	}
	data, err := os.ReadFile(filepath.Join(o.project, p.Workflow))
	if err != nil {
		return p, err
	}
	var w workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		return p, fmt.Errorf("workflow YAML: %w", err)
	}
	if len(w.Jobs) == 0 {
		return p, errors.New("workflow has no jobs")
	}
	selected := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("cyclic job dependencies: %s", id)
		}
		if selected[id] {
			return nil
		}
		j, ok := w.Jobs[id]
		if !ok {
			return fmt.Errorf("unknown job: %s", id)
		}
		visiting[id] = true
		if j.Needs.Kind != 0 {
			deps, err := stringList(j.Needs)
			if err != nil {
				return err
			}
			for _, dep := range deps {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		visiting[id] = false
		selected[id] = true
		return nil
	}
	if o.job != "" {
		err = visit(o.job)
	} else {
		for id := range w.Jobs {
			if err = visit(id); err != nil {
				break
			}
		}
	}
	if err != nil {
		return p, err
	}
	labels := map[string]bool{}
	for _, id := range sortedKeys(selected) {
		j := w.Jobs[id]
		if j.Uses != "" {
			return p, fmt.Errorf("job %s uses a reusable workflow; select a concrete workflow instead (not yet supported)", id)
		}
		if j.Container.Kind != 0 {
			return p, fmt.Errorf("job %s defines container: Nix runner image would be bypassed (not yet supported)", id)
		}
		resolved, err := runnerLabels(j)
		if err != nil {
			return p, fmt.Errorf("job %s: %w", id, err)
		}
		for _, label := range resolved {
			labels[label] = true
		}
	}
	for _, label := range sortedKeys(labels) {
		base := o.platforms[label]
		if base == "" {
			switch label {
			case "ubuntu-24.04", "ubuntu-latest", "ubuntu-24.04-arm":
				base = "ghcr.io/catthehacker/ubuntu:act-24.04"
			case "ubuntu-22.04", "ubuntu-22.04-arm":
				base = "ghcr.io/catthehacker/ubuntu:act-22.04"
			default:
				return p, fmt.Errorf("runner %q is unsupported; macOS/Windows cannot run in Linux containers; use --platform LABEL=IMAGE only for an intentional Linux substitute", label)
			}
		} else {
			p.Warnings = append(p.Warnings, "explicit runner substitution: "+label+" -> "+base)
		}
		if strings.Contains(label, "-arm") && o.arch != "linux/arm64" {
			return p, fmt.Errorf("runner %s requires --arch linux/arm64", label)
		}
		if label == "ubuntu-latest" {
			p.Warnings = append(p.Warnings, "ubuntu-latest mapped to Ubuntu 24.04; recheck GitHub runner image when its alias changes")
		}
		p.Images = append(p.Images, image{Label: label, Base: base, Nix: "nixos/nix:2.33.1"})
	}
	if o.arch == "linux/arm64" {
		p.Warnings = append(p.Warnings, "ARM64 is not equivalent to an x64 GitHub runner")
	}
	if o.dockerSocket {
		p.Warnings = append(p.Warnings, "Docker socket access enabled: workflow can control the Docker host")
	}
	return p, nil
}

var matrixExpr = regexp.MustCompile(`\$\{\{\s*matrix\.([A-Za-z_][A-Za-z_0-9]*)\s*\}\}`)

func runnerLabels(j job) ([]string, error) {
	values, err := stringList(j.RunsOn)
	if err != nil || len(values) == 0 {
		return nil, errors.New("unsupported runs-on form")
	}
	labels := []string{strings.Join(values, ",")}
	var matrix map[string]any
	if strings.Contains(labels[0], "${{") {
		if j.Strategy.Matrix.Kind != yaml.MappingNode {
			return nil, errors.New("dynamic runner requires a static strategy.matrix")
		}
		if err := j.Strategy.Matrix.Decode(&matrix); err != nil {
			return nil, err
		}
	}
	for len(labels) > 0 {
		expanded := false
		var next []string
		for _, label := range labels {
			match := matrixExpr.FindStringSubmatch(label)
			if len(match) == 0 {
				next = append(next, label)
				continue
			}
			key := match[1]
			candidates := map[string]bool{}
			if values, ok := matrix[key].([]any); ok {
				for _, v := range values {
					if s, ok := v.(string); ok {
						candidates[s] = true
					}
				}
			}
			if includes, ok := matrix["include"].([]any); ok {
				for _, v := range includes {
					if row, ok := v.(map[string]any); ok {
						if s, ok := row[key].(string); ok {
							candidates[s] = true
						}
					}
				}
			}
			if len(candidates) == 0 {
				return nil, fmt.Errorf("unresolved matrix runner: %s", key)
			}
			for _, s := range sortedKeys(candidates) {
				if strings.Contains(s, "${{") {
					return nil, errors.New("nested matrix expressions are unsupported")
				}
				next = append(next, strings.ReplaceAll(label, match[0], s))
			}
			expanded = true
		}
		labels = next
		if len(labels) > 128 {
			return nil, errors.New("runner matrix expands beyond 128 combinations")
		}
		if !expanded {
			break
		}
	}
	for _, label := range labels {
		if strings.Contains(label, "${{") {
			return nil, errors.New("unresolved dynamic runs-on expression")
		}
	}
	return labels, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func capture(ctx context.Context, dir, name string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	if name == "docker" && len(args) >= 2 && args[0] == "--host" {
		c.Env = dockerEndpointEnv(args[1])
	}
	c.Dir = dir
	// Do not include subprocess output in errors: Git URLs can contain credentials.
	b, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func excluded(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" || part == ".actrc" || part == ".env" || strings.HasPrefix(part, ".env.") && part != ".env.example" || part == ".secrets" || part == ".vars" || part == ".input" {
			return true
		}
	}
	return false
}

func snapshot(ctx context.Context, project, dest string) ([]string, error) {
	var err error
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return nil, err
	}
	root, err := capture(ctx, project, "git", "rev-parse", "--show-toplevel")
	if err != nil || root != project {
		return nil, errors.New("--project-dir must be the root of a Git checkout")
	}
	// No checkout: deleted tracked files stay deleted; overlay current tracked and untracked files.
	if _, err := capture(ctx, "", "git", "clone", "--no-hardlinks", "--no-checkout", "--", project, dest); err != nil {
		return nil, err
	}
	// Keep the source index (including staged additions) without copying its config
	// or checking out HEAD over the current worktree. Nix uses this tracked-file set.
	indexCmd := exec.CommandContext(ctx, "git", "ls-files", "--stage", "-z")
	indexCmd.Dir = project
	index, err := indexCmd.Output()
	if err != nil {
		return nil, err
	}
	for _, entry := range strings.Split(string(index), "\x00") {
		if strings.HasPrefix(entry, "160000 ") {
			return nil, errors.New("submodules are not yet supported; select an independent checkout")
		}
	}
	update := exec.CommandContext(ctx, "git", "update-index", "-z", "--index-info")
	update.Dir = dest
	update.Stdin = strings.NewReader(string(index))
	if err := update.Run(); err != nil {
		return nil, fmt.Errorf("snapshot index: %w", err)
	}
	origin, _ := capture(ctx, project, "git", "remote", "get-url", "origin")
	if origin != "" {
		u, err := url.Parse(origin)
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && (u.User != nil || u.RawQuery != "" || u.Fragment != "") {
			return nil, errors.New("origin URL contains credentials; remove credentials from URL before snapshotting")
		}
		if _, err := capture(ctx, dest, "git", "remote", "set-url", "origin", origin); err != nil {
			return nil, err
		}
	}
	c := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	c.Dir = project
	b, err := c.Output()
	if err != nil {
		return nil, err
	}
	var skipped []string
	for _, rel := range strings.Split(string(b), "\x00") {
		if rel == "" {
			continue
		}
		if !filepath.IsLocal(rel) {
			return skipped, errors.New("nonlocal path in Git file list")
		}
		if excluded(rel) {
			skipped = append(skipped, rel)
			continue
		}
		src, dst := filepath.Join(project, rel), filepath.Join(dest, rel)
		info, err := os.Lstat(src)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return skipped, err
		}
		if info.IsDir() {
			return skipped, fmt.Errorf("submodule/directory %s needs a separate initialized project (not yet supported)", rel)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return skipped, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(src)
			if err != nil {
				return skipped, err
			}
			if filepath.IsAbs(target) || !filepath.IsLocal(filepath.Join(filepath.Dir(rel), target)) {
				return skipped, fmt.Errorf("symlink escapes project: %s", rel)
			}
			if err := os.Symlink(target, dst); err != nil {
				return skipped, err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return skipped, fmt.Errorf("unsupported file type: %s", rel)
		}
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return skipped, err
		}
	}
	return skipped, nil
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

func actArgs(o options, p executionPlan, workspace, control, artifacts string) []string {
	args := []string{o.event, "-C", workspace, "-W", filepath.Join(workspace, p.Workflow), "--container-architecture", o.arch,
		"--env-file", filepath.Join(control, "empty"), "--input-file", filepath.Join(control, "empty"),
		"--secret-file", filepath.Join(control, "empty"), "--var-file", filepath.Join(control, "empty"),
		"--eventpath", filepath.Join(control, "event.json"), "--pull=false", "--concurrent-jobs", "1",
		"--use-gitignore=false", "--json", "--artifact-server-path", artifacts}
	var containerOptions []string
	if o.runID != "" {
		containerOptions = append(containerOptions, "--label=ci-self.run="+o.runID)
	}
	if p.NixCache != nil {
		containerOptions = append(containerOptions, "--volume="+p.NixCache.Store+":/nix", "--volume="+p.NixCache.Evaluation+":/root/.cache/nix")
	}
	if len(containerOptions) > 0 {
		args = append(args, "--container-options", strings.Join(containerOptions, " "))
	}
	// act's default COPY mode keeps working files off the original checkout.
	// Default checkout handling keeps local modifications; --no-skip-checkout would fetch remote HEAD.
	if !o.dockerSocket {
		args = append(args, "--container-daemon-socket", "-")
	}
	if o.secretFile != "" {
		args = append(args, "--secret-file", o.secretFile)
	}
	if o.varFile != "" {
		args = append(args, "--var-file", o.varFile)
	}
	if o.job != "" {
		args = append(args, "--job", o.job)
	}
	if o.cacheDir != "" {
		args = append(args, "--action-cache-path", filepath.Join(o.cacheDir, "act"), "--cache-server-path", filepath.Join(o.cacheDir, "actcache"))
	}
	for _, im := range p.Images {
		args = append(args, "--platform", im.Label+"="+im.Tag)
	}
	return args
}

func controlledEnv(control, dockerHost string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "HOME", "XDG_CONFIG_HOME", "DOCKER_CONFIG", "DOCKER_CONTEXT", "DOCKER_HOST", "GH_TOKEN", "GITHUB_TOKEN":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+control, "XDG_CONFIG_HOME="+control, "DOCKER_CONFIG="+control, "DOCKER_HOST="+dockerHost)
}

func run(ctx context.Context, args []string) (retErr error) {
	started := time.Now()
	o, err := parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	if o.list {
		wf, err := workflowPath(o.project, o.workflow)
		if err != nil {
			return err
		}
		c := exec.CommandContext(ctx, "act", o.event, "-C", o.project, "-W", filepath.Join(o.project, wf), "--container-architecture", o.arch, "--list")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}
	p, err := makePlan(o)
	if err != nil {
		return err
	}
	if o.plan {
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	for _, warning := range p.Warnings {
		fmt.Println("WARN:", warning)
	}
	for _, name := range []string{"git", "act"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s is required; enter nix develop first", name)
		}
	}
	if inside(o.project, o.cacheDir) {
		return errors.New("--cache-dir must be outside the target project")
	}
	if err := os.MkdirAll(o.cacheDir, 0o700); err != nil {
		return err
	}
	resolvedCache, err := filepath.EvalSymlinks(o.cacheDir)
	if err != nil {
		return err
	}
	if inside(o.project, resolvedCache) {
		return errors.New("--cache-dir resolves inside target project")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker is required")
	}
	dockerHost, dockerPlatform, disconnect, err := connectDocker(ctx, o.vm)
	if err != nil {
		return err
	}
	defer disconnect()
	if !strings.HasPrefix(dockerPlatform, "linux/") {
		return errors.New("sandbox requires a Linux Docker engine")
	}
	normalized := strings.NewReplacer("aarch64", "arm64", "x86_64", "amd64").Replace(dockerPlatform)
	if normalized != o.arch {
		warning := "CPU emulation: requested " + o.arch + ", engine " + dockerPlatform + "; Nix develop/build may fail seccomp; use a matching native engine, do not disable syscall filters"
		p.Warnings = append(p.Warnings, warning)
		fmt.Println("WARN:", warning)
	}
	if o.out == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		o.out = filepath.Join(cache, "ci-self-runner", "sandbox")
	}
	if inside(o.project, o.out) {
		return errors.New("--out must be outside the target project to avoid recursive snapshots")
	}
	if err := os.MkdirAll(o.out, 0o700); err != nil {
		return err
	}
	o.out, err = filepath.EvalSymlinks(o.out)
	if err != nil {
		return err
	}
	if inside(o.project, o.out) {
		return errors.New("--out resolves inside target project")
	}
	runDir, err := os.MkdirTemp(o.out, "run-")
	if err != nil {
		return err
	}
	runDir, err = filepath.Abs(runDir)
	if err != nil {
		return err
	}
	workspace, control, artifacts := filepath.Join(runDir, "project"), filepath.Join(runDir, "control"), filepath.Join(runDir, "artifacts")
	for _, dir := range []string{control, artifacts} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
	}
	log, err := os.OpenFile(filepath.Join(runDir, "run.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	output := io.MultiWriter(os.Stdout, log)
	r := result{Plan: p, Started: started.UTC().Format(time.RFC3339), Status: "ERROR", Phase: "snapshot", DockerPlatform: dockerPlatform, ElapsedSeconds: map[string]float64{"controller_setup": time.Since(started).Seconds()}}
	phaseStarted := time.Now()
	beginPhase := func(phase string) {
		r.ElapsedSeconds[r.Phase] += time.Since(phaseStarted).Seconds()
		r.Phase = phase
		phaseStarted = time.Now()
	}
	defer func() {
		r.ElapsedSeconds[r.Phase] += time.Since(phaseStarted).Seconds()
		r.ElapsedSeconds["total"] = time.Since(started).Seconds()
		if retErr != nil {
			r.Error = retErr.Error()
		}
		r.Finished = time.Now().UTC().Format(time.RFC3339)
		b, _ := json.MarshalIndent(r, "", "  ")
		if err := os.WriteFile(filepath.Join(runDir, "result.json"), b, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: write result:", err)
			retErr = errors.Join(retErr, err)
		}
	}()
	fmt.Fprintln(output, "OK: run_dir="+runDir)
	r.Excluded, err = snapshot(ctx, o.project, workspace)
	if err != nil {
		r.Error = err.Error()
		return err
	}
	for _, rel := range r.Excluded {
		fmt.Fprintln(output, "SKIP: snapshot excluded="+rel)
	}
	if _, err := os.Stat(filepath.Join(workspace, p.Workflow)); err != nil {
		r.Error = "workflow excluded from snapshot"
		return errors.New(r.Error)
	}
	if err := os.WriteFile(filepath.Join(control, "empty"), nil, 0o600); err != nil {
		return err
	}
	event := []byte("{\"act\":true}\n")
	if o.eventFile != "" {
		event, err = os.ReadFile(o.eventFile)
		if err != nil {
			return err
		}
		if !json.Valid(event) {
			return errors.New("invalid event JSON")
		}
	}
	if err := os.WriteFile(filepath.Join(control, "event.json"), event, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(control, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		return err
	}
	r.ActVersion, _ = capture(ctx, "", "act", "--version")
	env := controlledEnv(control, dockerHost)
	execute := func(name string, arguments ...string) error {
		c := exec.CommandContext(ctx, name, arguments...)
		c.Cancel = func() error { return c.Process.Signal(os.Interrupt) }
		c.WaitDelay = 10 * time.Second
		c.Dir = control
		c.Env = env
		c.Stdout, c.Stderr = output, output
		return c.Run()
	}
	// Pull once, record immutable digests, then build only from those inputs.
	beginPhase("images")
	for i := range r.Plan.Images {
		im := &r.Plan.Images[i]
		for _, ref := range []*string{&im.Base, &im.Nix} {
			if err := execute("docker", "pull", "--platform", o.arch, *ref); err != nil {
				r.Error = "image pull failed"
				return fmt.Errorf("image pull: %w", err)
			}
			resolved, err := capture(ctx, "", "docker", "--host", dockerHost, "image", "inspect", "--format", "{{index .RepoDigests 0}}", *ref)
			if err != nil {
				return err
			}
			*ref = resolved
		}
		hash := sha256.Sum256([]byte(im.Base + im.Nix + o.arch + dockerfile))
		im.Tag = "ci-self-sandbox:" + hex.EncodeToString(hash[:12])
		if err := execute("docker", "build", "--platform", o.arch, "--build-arg", "BASE_IMAGE="+im.Base, "--build-arg", "NIX_IMAGE="+im.Nix, "-t", im.Tag, control); err != nil {
			r.Error = "image build failed"
			return fmt.Errorf("image build: %w", err)
		}
		im.ID, err = capture(ctx, "", "docker", "--host", dockerHost, "image", "inspect", "--format", "{{.Id}}", im.Tag)
		if err != nil {
			return err
		}
	}
	o.runID = filepath.Base(runDir)
	if o.nixCache {
		beginPhase("nix_cache")
		cache := nixCacheFor(o.project, o.arch, r.Plan.Images)
		r.Plan.NixCache = &cache
		engineID, err := capture(ctx, "", "docker", "--host", dockerHost, "info", "--format", "{{.ID}}")
		if err != nil {
			return err
		}
		if engineID == "" {
			return errors.New("Docker engine identity is unavailable; cannot lock Nix cache safely")
		}
		unlock, err := lockNixCache(cache.Key, engineID)
		if err != nil {
			return err
		}
		defer unlock()
		for _, volume := range []string{cache.Store, cache.Evaluation} {
			if err := execute("docker", "volume", "create", "--label", "ci-self.cache-key="+cache.Key, volume); err != nil {
				return err
			}
			key, err := capture(ctx, "", "docker", "--host", dockerHost, "volume", "inspect", "--format", "{{index .Labels \"ci-self.cache-key\"}}", volume)
			if err != nil {
				return err
			}
			if key != cache.Key {
				return errors.New("Nix cache volume ownership label mismatch: " + volume)
			}
		}
		active, err := capture(ctx, "", "docker", "--host", dockerHost, "ps", "--filter", "volume="+cache.Store, "--format", "{{.ID}}")
		if err != nil {
			return err
		}
		if active != "" {
			return fmt.Errorf("Nix cache is mounted by running container(s) %s; inspect the previous run first", active)
		}
		fmt.Fprintf(output, "OK: persistent Nix cache store=%s evaluation=%s\n", cache.Store, cache.Evaluation)
	}
	r.RunLabel = "ci-self.run=" + o.runID
	defer func() {
		stopped, err := stopRunContainers(ctx, dockerHost, o.runID, output)
		r.StoppedContainers = stopped
		if err != nil {
			r.Status = "ERROR"
			retErr = errors.Join(retErr, fmt.Errorf("stop leftover containers: %w", err))
		}
	}()
	beginPhase("workflow")
	actErr := execute("act", actArgs(o, r.Plan, workspace, control, artifacts)...)
	r.Jobs, err = readJobOutcomes(filepath.Join(runDir, "run.log"))
	if actErr != nil {
		r.Error = "workflow failed or cancelled"
		return fmt.Errorf("workflow: %w (results: %s)", actErr, runDir)
	}
	if err != nil {
		return err
	}
	completed := 0
	for _, j := range r.Jobs {
		if j.Result == "success" && (o.job == "" || j.ID == o.job) {
			completed++
		}
	}
	if completed == 0 {
		r.Status = "SKIPPED"
		return errors.New("no selected job completed successfully; check event, job conditions and run.log")
	}
	r.Status = "OK"
	beginPhase("complete")
	fmt.Fprintf(output, "OK: act completed; selected successful job variants=%d; results=%s\n", completed, runDir)
	return nil
}

func readJobOutcomes(path string) ([]jobOutcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 8*1024*1024)
	jobs := map[string]jobOutcome{}
	for s.Scan() {
		var entry struct {
			ID     string         `json:"jobID"`
			Matrix map[string]any `json:"matrix"`
			Result string         `json:"jobResult"`
			Raw    bool           `json:"raw_output"`
		}
		if json.Unmarshal(s.Bytes(), &entry) != nil || entry.Raw || entry.ID == "" || entry.Result == "" {
			continue
		}
		matrix, _ := json.Marshal(entry.Matrix)
		jobs[entry.ID+string(matrix)] = jobOutcome{ID: entry.ID, Matrix: entry.Matrix, Result: entry.Result}
	}
	var outcomes []jobOutcome
	for _, key := range sortedKeys(jobs) {
		outcomes = append(outcomes, jobs[key])
	}
	return outcomes, s.Err()
}

func inside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}
