package ci_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runScaffoldMobileBuildWorkflow(t *testing.T, repo string, args ...string) (string, error) {
	t.Helper()
	allArgs := []string{"./scaffold_mobile_build_workflow.sh", "--repo", repo}
	allArgs = append(allArgs, args...)
	cmd := exec.Command("bash", allArgs...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestScaffoldMobileBuildWorkflowPrintsFastlaneTemplate(t *testing.T) {
	repo := t.TempDir()
	out, err := runScaffoldMobileBuildWorkflow(t, repo)
	if err != nil {
		t.Fatalf("scaffold dry-run failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		`branches: ["main", "develop"]`,
		"bundle exec fastlane ios",
		"bundle exec fastlane android",
		"actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5",
		"- mobile",
		"- ios",
		"- android",
		"- xcode",
		"- android-sdk",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated workflow missing %q\ncontent:\n%s", want, out)
		}
	}
}

func TestScaffoldMobileBuildWorkflowCreatesWhenMissing(t *testing.T) {
	repo := t.TempDir()
	out, err := runScaffoldMobileBuildWorkflow(t, repo, "--apply")
	if err != nil {
		t.Fatalf("scaffold failed: %v\noutput:\n%s", err, out)
	}

	target := filepath.Join(repo, ".github", "workflows", "mobile-build.yml")
	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("expected mobile workflow to be created: %v", readErr)
	}
	if !strings.Contains(string(body), "name: mobile-build") {
		t.Fatalf("unexpected workflow content:\n%s", string(body))
	}
	if strings.Contains(string(body), "actions/checkout@"+"v4") {
		t.Fatalf("checkout action must be SHA-pinned\ncontent:\n%s", string(body))
	}
	if !strings.Contains(out, "OK: wrote "+target) {
		t.Fatalf("expected write output\noutput:\n%s", out)
	}
}

func TestScaffoldMobileBuildWorkflowSkipsWhenExists(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".github", "workflows", "mobile-build.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	const keep = "KEEP_EXISTING_WORKFLOW\n"
	if err := os.WriteFile(target, []byte(keep), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out, err := runScaffoldMobileBuildWorkflow(t, repo, "--apply")
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
