package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, content string) options {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, ".github/workflows/ci.yml"), content)
	return options{project: root, event: "push", arch: "linux/amd64", platforms: mappings{}}
}

func TestPlanSelectionIncludesNeedsButNotUnrelatedOS(t *testing.T) {
	o := fixture(t, `on: push
jobs:
  prepare:
    runs-on: ubuntu-22.04
  test:
    needs: prepare
    runs-on: ubuntu-24.04
  mac:
    runs-on: macos-latest
`)
	o.job = "test"
	p, err := makePlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Images) != 2 {
		t.Fatalf("dependency omitted: %+v", p)
	}
	o.job = ""
	if _, err := makePlan(o); err == nil {
		t.Fatal("must reject unselected unsupported OS")
	}
}

func TestRelativeOptionsAreResolvedFromCallerNotRunnerCheckout(t *testing.T) {
	caller := t.TempDir()
	caller, err := filepath.EvalSymlinks(caller)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(caller, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_SELF_CALLER_DIR", caller)
	o, err := parse([]string{"--project-dir", "project", "--secret-file", "secrets.env", "--out", "results"})
	if err != nil {
		t.Fatal(err)
	}
	if o.project != filepath.Join(caller, "project") || o.secretFile != filepath.Join(caller, "secrets.env") || o.out != filepath.Join(caller, "results") {
		t.Fatalf("wrong base: %+v", o)
	}
}

func TestVMOptionIsRecordedWithoutConnectingDuringPlan(t *testing.T) {
	o := fixture(t, "on: push\njobs:\n  test:\n    runs-on: ubuntu-24.04\n")
	o.vm = "ci-self-x64"
	p, err := makePlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.VM != o.vm {
		t.Fatal("VM selection missing from plan")
	}
	if _, err := parse([]string{"--project-dir", o.project, "--vm", "../bad"}); err == nil {
		t.Fatal("unsafe instance name accepted")
	}
}

func TestPlanRejectsUnsupportedWorkflowForms(t *testing.T) {
	for _, body := range []string{
		"runs-on: macos-latest",
		"runs-on: [self-hosted, mac-mini]",
		"runs-on: '${{ inputs.runner }}'",
		"uses: owner/repo/.github/workflows/ci.yml@main",
		"runs-on: ubuntu-24.04\n    container: node:22",
		"runs-on: ubuntu-24.04\n    needs: missing",
		"runs-on: ubuntu-24.04\n    needs: test",
	} {
		o := fixture(t, "on: push\njobs:\n  test:\n    "+body+"\n")
		if _, err := makePlan(o); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestPlanExpandsStaticRunnerMatrixAndRejectsWrongArch(t *testing.T) {
	o := fixture(t, `on: push
jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-22.04, ubuntu-24.04]
`)
	p, err := makePlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Images) != 2 {
		t.Fatalf("matrix not expanded: %+v", p)
	}
	o = fixture(t, "on: push\njobs:\n  test:\n    runs-on: ubuntu-24.04-arm\n")
	if _, err := makePlan(o); err == nil {
		t.Fatal("wrong arch accepted")
	}
	o.arch = "linux/arm64"
	if _, err := makePlan(o); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowMustBeExplicitWhenAmbiguousAndStayInsideProject(t *testing.T) {
	o := fixture(t, "jobs: {}")
	write(t, filepath.Join(o.project, ".github/workflows/other.yaml"), "jobs: {}")
	if _, err := workflowPath(o.project, ""); err == nil {
		t.Fatal("ambiguous workflow accepted")
	}
	external := filepath.Join(t.TempDir(), "outside.yml")
	write(t, external, "jobs: {}")
	if _, err := workflowPath(o.project, external); err == nil {
		t.Fatal("external workflow accepted")
	}
}

func TestSnapshotCopiesCurrentWorktreeWithoutSourceCredentials(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := capture(ctx, src, "git", args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	git("init", "-q")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "test")
	write(t, filepath.Join(src, "tracked.txt"), "committed")
	write(t, filepath.Join(src, ".gitignore"), "cache/\n")
	git("add", ".")
	// Add an index entry without materializing it: a tracked file deleted in the worktree.
	blob := git("hash-object", "-w", "tracked.txt")
	git("update-index", "--add", "--cacheinfo", "100644,"+blob+",deleted.txt")
	git("commit", "-qm", "fixture")
	git("remote", "add", "origin", "https://github.com/example/project.git")
	git("config", "http.extraheader", "sensitive-test-marker")
	write(t, filepath.Join(src, "tracked.txt"), "modified")
	write(t, filepath.Join(src, "untracked.txt"), "new")
	write(t, filepath.Join(src, "staged.txt"), "staged")
	git("add", "staged.txt")
	write(t, filepath.Join(src, "cache/ignored.txt"), "ignored")
	write(t, filepath.Join(src, ".env"), "SECRET=test")
	write(t, filepath.Join(src, ".actrc"), "--bind")
	dst := filepath.Join(t.TempDir(), "project")
	skipped, err := snapshot(ctx, src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 2 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	for path, want := range map[string]string{"tracked.txt": "modified", "untracked.txt": "new"} {
		b, err := os.ReadFile(filepath.Join(dst, path))
		if err != nil || string(b) != want {
			t.Fatalf("snapshot %s: %s %v", path, b, err)
		}
	}
	for _, path := range []string{"deleted.txt", "cache/ignored.txt", ".env", ".actrc"} {
		if _, err := os.Stat(filepath.Join(dst, path)); !os.IsNotExist(err) {
			t.Fatalf("unexpected snapshot file %s: %v", path, err)
		}
	}
	config, err := os.ReadFile(filepath.Join(dst, ".git/config"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "sensitive-test-marker") {
		t.Fatal("source Git credentials copied")
	}
	b, _ := os.ReadFile(filepath.Join(src, "tracked.txt"))
	if string(b) != "modified" {
		t.Fatal("source modified")
	}
	if _, err := capture(ctx, dst, "git", "log", "-1", "--oneline"); err != nil {
		t.Fatal("Git history missing:", err)
	}
	tracked, err := capture(ctx, dst, "git", "ls-files")
	if err != nil || !strings.Contains(tracked, "staged.txt") || !strings.Contains(tracked, "tracked.txt") {
		t.Fatalf("source index lost: %s %v", tracked, err)
	}
}

func TestSnapshotRejectsEscapingSymlink(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	if _, err := capture(ctx, src, "git", "init", "-q"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(src, "file"), "x")
	if _, err := capture(ctx, src, "git", "add", "file"); err != nil {
		t.Fatal(err)
	}
	// clone needs a commit; use an empty commit with explicit local identity.
	if _, err := capture(ctx, src, "git", "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(src, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot(ctx, src, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("escaping symlink accepted")
	}
}

func TestActArgumentsKeepLocalChangesAndDoNotMountHost(t *testing.T) {
	o := options{event: "push", arch: "linux/amd64", cacheDir: "/cache"}
	p := executionPlan{Workflow: ".github/workflows/ci.yml", Images: []image{{Label: "ubuntu-24.04", Tag: "test-image"}}}
	args := strings.Join(actArgs(o, p, "/copy", "/control", "/artifacts"), " ")
	for _, bad := range []string{"--bind", "--no-skip-checkout", "=-self-hosted", "--privileged"} {
		if strings.Contains(args, bad) {
			t.Fatalf("unsafe argument %s", args)
		}
	}
	for _, want := range []string{"--container-daemon-socket -", "--secret-file /control/empty", "ubuntu-24.04=test-image", "--cache-server-path /cache/actcache"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %s: %s", want, args)
		}
	}
	t.Setenv("GITHUB_TOKEN", "never-forward-this")
	t.Setenv("DOCKER_CONTEXT", "other")
	env := strings.Join(controlledEnv("/control", "unix:///test.sock"), "\n")
	if strings.Contains(env, "never-forward-this") || strings.Contains(env, "DOCKER_CONTEXT=") {
		t.Fatal("host credentials/context inherited")
	}
}

func TestReadJobOutcomesDoesNotTreatWorkflowOutputAsRunnerEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	write(t, path, `docker build output
{"jobID":"test","matrix":{"go":"1.25"},"jobResult":"success"}
{"jobID":"test","matrix":{"go":"1.26"},"jobResult":"failure"}
{"jobID":"forged","jobResult":"success","raw_output":true}
{"msg":"{\"jobID\":\"forged\",\"jobResult\":\"success\"}","raw_output":true}
`)
	jobs, err := readJobOutcomes(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].ID != "test" || jobs[1].Result != "failure" {
		t.Fatalf("unexpected outcomes: %+v", jobs)
	}
	write(t, filepath.Join(t.TempDir(), "empty"), "")
	path = filepath.Join(t.TempDir(), "skipped.log")
	write(t, path, `{"jobID":"test","jobResult":"skipped"}`)
	jobs, err = readJobOutcomes(path)
	if err != nil || len(jobs) != 1 || jobs[0].Result != "skipped" {
		t.Fatalf("skipped result lost: %+v %v", jobs, err)
	}
}
