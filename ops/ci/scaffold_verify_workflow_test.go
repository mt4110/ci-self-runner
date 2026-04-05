package ci_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runScaffoldVerifyWorkflow(t *testing.T, repo string, args ...string) (string, error) {
	t.Helper()
	return runScaffoldVerifyWorkflowWithEnvInput(t, repo, nil, "", args...)
}

func runScaffoldVerifyWorkflowWithEnvInput(t *testing.T, repo string, env []string, input string, args ...string) (string, error) {
	t.Helper()
	allArgs := []string{"./scaffold_verify_workflow.sh", "--repo", repo}
	allArgs = append(allArgs, args...)
	cmd := exec.Command("bash", allArgs...)
	cmd.Dir = "."
	if len(env) > 0 {
		cmd.Env = mergeEnvOverrides(os.Environ(), env)
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runScaffoldVerifyWorkflowTTY(t *testing.T, repo string, input string, args ...string) (string, error) {
	t.Helper()
	env := []string{"CI_SELF_TEST_FORCE_TTY=1"}
	return runScaffoldVerifyWorkflowWithEnvInput(t, repo, env, input, args...)
}

func TestScaffoldVerifyWorkflowCreatesWhenMissingNonInteractive(t *testing.T) {
	repo := t.TempDir()

	out, err := runScaffoldVerifyWorkflow(t, repo, "--apply")
	if err != nil {
		t.Fatalf("scaffold failed: %v\noutput:\n%s", err, out)
	}

	target := filepath.Join(repo, ".github", "workflows", "verify.yml")
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("expected verify workflow to be created: %v", statErr)
	}
	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read failed: %v", readErr)
	}
	if !strings.Contains(string(body), "- uses: actions/checkout@v4") {
		t.Fatalf("expected checkout step in generated workflow\ncontent:\n%s", string(body))
	}
	if strings.Contains(string(body), "if: ${{ !env.ACT }}\n        uses: actions/checkout@v4") {
		t.Fatalf("checkout step should not be guarded by ACT\ncontent:\n%s", string(body))
	}
	if !strings.Contains(out, "OK: wrote "+target) {
		t.Fatalf("expected write output\noutput:\n%s", out)
	}
}

func TestScaffoldVerifyWorkflowSkipsWhenExistsNonInteractive(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "workflows", "verify.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	const keep = "KEEP_EXISTING_WORKFLOW\n"
	if err := os.WriteFile(target, []byte(keep), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldVerifyWorkflow(t, repo, "--apply")
	if err != nil {
		t.Fatalf("expected skip success, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "SKIP: "+target+" already exists") {
		t.Fatalf("expected skip output\noutput:\n%s", out)
	}

	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read failed: %v", readErr)
	}
	if string(body) != keep {
		t.Fatalf("existing workflow should be preserved\ncontent:\n%s", string(body))
	}
}

func TestScaffoldVerifyWorkflowPromptsBeforeCreateOnTTY(t *testing.T) {
	repo := t.TempDir()

	out, err := runScaffoldVerifyWorkflowTTY(t, repo, "y\n", "--apply")
	if err != nil {
		t.Fatalf("tty scaffold failed: %v\noutput:\n%s", err, out)
	}

	target := filepath.Join(repo, ".github", "workflows", "verify.yml")
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("expected verify workflow to be created: %v", statErr)
	}
	if !strings.Contains(out, "verify.yml がありません。作成しますか？ [y/N]") {
		t.Fatalf("expected create prompt\noutput:\n%s", out)
	}
}

func TestScaffoldVerifyWorkflowPromptsBeforeOverwriteOnTTY(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "workflows", "verify.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(target, []byte("OLD\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldVerifyWorkflowTTY(t, repo, "y\n", "--apply")
	if err != nil {
		t.Fatalf("tty overwrite failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "verify.yml を上書きしますか？ [y/N]") {
		t.Fatalf("expected overwrite prompt\noutput:\n%s", out)
	}

	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read failed: %v", readErr)
	}
	if !strings.Contains(string(body), "jobs:") {
		t.Fatalf("expected workflow content to be overwritten\ncontent:\n%s", string(body))
	}
}

func TestScaffoldVerifyWorkflowDeclineOverwriteOnTTY(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "workflows", "verify.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	const keep = "KEEP_EXISTING_WORKFLOW\n"
	if err := os.WriteFile(target, []byte(keep), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldVerifyWorkflowTTY(t, repo, "n\n", "--apply")
	if err != nil {
		t.Fatalf("expected decline overwrite to exit successfully, got: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "SKIP: user declined workflow overwrite") {
		t.Fatalf("expected decline output\noutput:\n%s", out)
	}

	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read failed: %v", readErr)
	}
	if string(body) != keep {
		t.Fatalf("existing workflow should remain unchanged\ncontent:\n%s", string(body))
	}
}
