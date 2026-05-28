package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type options struct {
	apply       bool
	repo        string
	installDir  string
	skipNetwork bool
	runnerScan  bool
}

type dependency struct {
	name        string
	bin         string
	brewPackage string
	installHint string
	upgradeHint string
}

var dependencies = []dependency{
	{name: "act", bin: "act", brewPackage: "act", installHint: "brew install act", upgradeHint: "brew update && brew upgrade act"},
	{name: "gh", bin: "gh", brewPackage: "gh", installHint: "brew install gh", upgradeHint: "brew update && brew upgrade gh"},
	{name: "colima", bin: "colima", brewPackage: "colima", installHint: "brew install colima", upgradeHint: "brew update && brew upgrade colima"},
	{name: "docker", bin: "docker", brewPackage: "docker", installHint: "install Docker Desktop or brew install docker", upgradeHint: "review Docker Desktop updates or brew upgrade docker"},
	{name: "go", bin: "go", brewPackage: "go", installHint: "brew install go or mise install", upgradeHint: "brew upgrade go or update mise.toml intentionally"},
	{name: "mise", bin: "mise", brewPackage: "mise", installHint: "brew install mise", upgradeHint: "brew update && brew upgrade mise"},
	{name: "rsync", bin: "rsync", brewPackage: "rsync", installHint: "brew install rsync", upgradeHint: "brew update && brew upgrade rsync"},
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		printUsage()
		fmt.Printf("ERROR: update invalid_args=%s\n", err.Error())
		fmt.Println("STATUS: ERROR")
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Println("STATUS: ERROR")
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		repo:       firstNonEmpty(os.Getenv("CI_SELF_REPO"), os.Getenv("RUNNER_REPO")),
		runnerScan: true,
	}
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printUsage()
			os.Exit(0)
		}
	}

	fs := flag.NewFlagSet("runner_update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.apply, "apply", false, "upgrade already-installed Homebrew-managed tools")
	fs.Bool("check", false, "advisory check only")
	fs.StringVar(&opts.repo, "repo", "", "target repository (owner/repo)")
	fs.StringVar(&opts.installDir, "install-dir", "", "runner install directory")
	fs.StringVar(&opts.installDir, "runner-dir", "", "runner install directory")
	fs.BoolVar(&opts.skipNetwork, "skip-network", false, "skip network lookups")
	noRunnerScan := fs.Bool("no-runner-scan", false, "skip runner directory scan")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected_args=%s", strings.Join(fs.Args(), ","))
	}
	opts.runnerScan = !*noRunnerScan
	return opts, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func printUsage() {
	fmt.Println("Usage: ci-self update [--apply] [--repo owner/repo] [--install-dir path] [--skip-network] [--no-runner-scan]")
	fmt.Println()
	fmt.Println("Checks the GitHub Actions runner version and brew-managed dependencies.")
	fmt.Println("By default this is advisory only. --apply upgrades only already-installed Homebrew packages.")
}

func run(opts options) error {
	fmt.Printf("OK: update start apply=%d\n", boolInt(opts.apply))

	failed := false
	if opts.runnerScan {
		checkRunnerUpdates(opts.repo, opts.installDir, opts.skipNetwork)
		if opts.apply {
			fmt.Println("SKIP: update apply=runner reason=runner_self_updates_are_managed_by_github_actions")
		}
	}

	brewReady := commandAvailable("brew")
	if opts.apply && brewReady {
		fmt.Println("OK: update apply=brew_update")
		if err := runStreaming("brew", "update"); err != nil {
			fmt.Println("ERROR: update apply=brew_update reason=failed")
			failed = true
			brewReady = false
		}
	}

	for _, dep := range dependencies {
		if err := checkDependency(dep, opts.apply && brewReady); err != nil {
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("update_failed")
	}
	fmt.Println("STATUS: OK")
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func checkRunnerUpdates(repo, installDir string, skipNetwork bool) {
	latest := ""
	if !skipNetwork {
		latest = latestGitHubReleaseTag("actions/runner")
	}
	latestClean := normalizeSemver(latest)

	dirs := collectRunnerDirs(repo, installDir)
	if len(dirs) == 0 {
		fmt.Println("SKIP: update check=runner reason=no_runner_dirs_found")
		return
	}

	for _, dir := range dirs {
		listener := runnerListenerPath(dir)
		if listener == "" {
			fmt.Printf("WARN: update check=runner dir=%s reason=listener_not_found\n", dir)
			continue
		}
		raw, _ := firstLine(listener, "--version")
		current := normalizeSemver(raw)
		if current == "" {
			fmt.Printf("WARN: update check=runner dir=%s reason=version_unknown\n", dir)
			continue
		}
		if latestClean == "" {
			fmt.Printf("OK: update check=runner dir=%s current=v%s latest=unknown reason=network_skipped_or_unavailable\n", dir, current)
			continue
		}
		if semverLT(current, latestClean) {
			fmt.Printf("WARN: update check=runner dir=%s current=v%s latest=v%s hint=runner_auto_update_should_catch_up_next_job_or_within_a_week\n", dir, current, latestClean)
			continue
		}
		fmt.Printf("OK: update check=runner dir=%s current=v%s latest=v%s\n", dir, current, latestClean)
	}
}

func checkDependency(dep dependency, apply bool) error {
	if !commandAvailable(dep.bin) {
		if dep.name == "go" && commandAvailable("mise") {
			if miseGo, _ := firstLine("mise", "x", "--", "go", "version"); miseGo != "" {
				fmt.Printf("OK: update check=dependency name=go current=%q latest=unknown reason=available_via_mise hint=\"update mise.toml intentionally\"\n", miseGo)
				return nil
			}
		}
		fmt.Printf("WARN: update check=dependency name=%s reason=missing hint=%q\n", dep.name, dep.installHint)
		return nil
	}

	current := dependencyCurrentVersion(dep)
	if current == "" {
		current = "unknown"
	}
	if !commandAvailable("brew") {
		fmt.Printf("OK: update check=dependency name=%s current=%q latest=unknown reason=brew_not_available\n", dep.name, current)
		return nil
	}
	if err := runQuiet("brew", "list", "--versions", dep.brewPackage); err != nil {
		fmt.Printf("OK: update check=dependency name=%s current=%q latest=unknown reason=not_brew_managed hint=%q\n", dep.name, current, dep.upgradeHint)
		return nil
	}

	outdated, err := output("brew", "outdated", "--quiet", dep.brewPackage)
	outdated = strings.TrimSpace(outdated)
	if err != nil {
		if outdated == "" {
			fmt.Printf("WARN: update check=dependency name=%s current=%q latest=unknown reason=brew_outdated_failed\n", dep.name, current)
			return nil
		}
	}
	if outdated == "" {
		fmt.Printf("OK: update check=dependency name=%s current=%q latest=brew_current\n", dep.name, current)
		return nil
	}

	fmt.Printf("WARN: update check=dependency name=%s current=%q latest=brew_outdated hint=\"brew update && brew upgrade %s\"\n", dep.name, current, dep.brewPackage)
	if !apply {
		return nil
	}
	fmt.Printf("OK: update apply=brew_upgrade package=%s\n", dep.brewPackage)
	if err := runStreaming("brew", "upgrade", dep.brewPackage); err != nil {
		fmt.Printf("ERROR: update apply=brew_upgrade package=%s reason=failed\n", dep.brewPackage)
		return err
	}
	return nil
}

func dependencyCurrentVersion(dep dependency) string {
	switch dep.name {
	case "colima":
		v, _ := firstLine(dep.bin, "version")
		return v
	case "go":
		v, _ := firstLine(dep.bin, "version")
		return v
	default:
		v, _ := firstLine(dep.bin, "--version")
		return v
	}
}

func latestGitHubReleaseTag(repo string) string {
	if repo == "actions/runner" {
		if v := strings.TrimSpace(os.Getenv("CI_SELF_UPDATE_LATEST_RUNNER_TAG")); v != "" {
			return v
		}
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest") //nolint:gosec // repo is a fixed owner/name in current use.
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.TagName)
}

func collectRunnerDirs(repo, installDir string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" || seen[dir] || !isDir(dir) {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}

	if installDir != "" {
		add(expandLocalPath(installDir))
	} else if repo != "" {
		add(runnerInstallDirForRepo(repo))
	}

	home := os.Getenv("HOME")
	if home == "" {
		return out
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".local", "ci-runner*"))
	for _, match := range matches {
		add(match)
	}
	return out
}

func runnerInstallDirForRepo(repo string) string {
	base := filepath.Join(os.Getenv("HOME"), ".local")
	if repo == "" {
		return filepath.Join(base, "ci-runner")
	}
	return filepath.Join(base, "ci-runner-"+sanitizeForPath(repo))
}

func sanitizeForPath(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if allowed {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}

func runnerListenerPath(dir string) string {
	candidates := []string{
		filepath.Join(dir, "bin", "Runner.Listener"),
		filepath.Join(dir, "Runner.Listener"),
	}
	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate
		}
	}
	return ""
}

func expandLocalPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(path, "~/"))
	}
	return path
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "act version ")
	v = strings.TrimPrefix(v, "v")
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return ""
	}
	v = fields[0]
	v = strings.SplitN(v, "-", 2)[0]
	return v
}

func semverLT(a, b string) bool {
	aa := semverParts(a)
	bb := semverParts(b)
	for i := 0; i < 3; i++ {
		if aa[i] < bb[i] {
			return true
		}
		if aa[i] > bb[i] {
			return false
		}
	}
	return false
}

var leadingDigits = regexp.MustCompile(`^\d+`)

func semverParts(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		raw := leadingDigits.FindString(parts[i])
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err == nil {
			out[i] = n
		}
	}
	return out
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func firstLine(name string, args ...string) (string, error) {
	out, err := output(name, args...)
	line := strings.TrimSpace(strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")[0])
	return line, err
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func runQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func runStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}
