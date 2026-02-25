package ci_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runScaffoldPRTemplate(t *testing.T, repo string, args ...string) (string, error) {
	t.Helper()
	allArgs := []string{"./scaffold_pr_template.sh", "--repo", repo}
	allArgs = append(allArgs, args...)
	cmd := exec.Command("bash", allArgs...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestScaffoldPRTemplateCreatesCanonicalTemplate(t *testing.T) {
	repo := t.TempDir()
	out, err := runScaffoldPRTemplate(t, repo, "--apply")
	if err != nil {
		t.Fatalf("scaffold failed: %v\noutput:\n%s", err, out)
	}
	target := filepath.Join(repo, ".github", "pull_request_template.md")
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read generated template: %v", err)
	}
	if !strings.Contains(string(b), "## Summary") {
		t.Fatalf("template content missing expected section\ncontent:\n%s", string(b))
	}
}

func TestScaffoldPRTemplateSkipsWhenTemplateExists(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "pull_request_template.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	const keep = "KEEP_EXISTING_TEMPLATE\n"
	if err := os.WriteFile(target, []byte(keep), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldPRTemplate(t, repo, "--apply")
	if err != nil {
		t.Fatalf("expected skip success, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "SKIP: pr template already exists") {
		t.Fatalf("expected skip message\noutput:\n%s", out)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(b) != keep {
		t.Fatalf("existing template should be preserved\ncontent:\n%s", string(b))
	}
}

func TestScaffoldPRTemplateForceOverwritesExisting(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "pull_request_template.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(target, []byte("OLD\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldPRTemplate(t, repo, "--apply", "--force")
	if err != nil {
		t.Fatalf("force scaffold failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "OK: wrote") {
		t.Fatalf("expected write message\noutput:\n%s", out)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(string(b), "## Verification") {
		t.Fatalf("forced content not written\ncontent:\n%s", string(b))
	}
}

