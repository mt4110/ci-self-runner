package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormattingChecksRepositorySourcesButPreservesIgnoredSnapshots(t *testing.T) {
	t.Chdir(t.TempDir())
	ctx := context.Background()
	for path, content := range map[string]string{
		".gitignore":  "out/\n",
		"tracked.go":  "package main\nfunc tracked( ) {}\n",
		"new file.go": "package main\nfunc added( ) {}\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "tracked.go"}} {
		if b, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, b)
		}
	}
	if err := os.MkdirAll("out/run/project", 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := []byte("package main\nfunc snapshot( ) {}\n")
	if err := os.WriteFile("out/run/project/foreign.go", snapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := goFormattingFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("formatting inputs: %v", files)
	}
	out, err := runCommandCapture(ctx, "gofmt", append([]string{"-l", "--"}, files...)...)
	if err != nil || !strings.Contains(out, "tracked.go") || !strings.Contains(out, "new file.go") || strings.Contains(out, "foreign.go") {
		t.Fatalf("formatting missed source or included evidence: %q %v", out, err)
	}
	if b, err := os.ReadFile("out/run/project/foreign.go"); err != nil || string(b) != string(snapshot) {
		t.Fatal("snapshot changed")
	}
}

func TestIsMobileSensitivePath(t *testing.T) {
	for _, path := range []string{
		"ios/signing/dist.p12",
		"ios/profile.mobileprovision",
		"android/release.jks",
		"android/key.properties",
		"fastlane/.env",
		"fastlane/.env.production",
		"apps/mobile/fastlane/.env.production",
		"AuthKey_ABC123.p8",
		"authkey_abc123.p8",
		"ios/asc_api_key.p8",
		"secrets/custom_name.P8",
	} {
		if !isMobileSensitivePath(path) {
			t.Fatalf("expected sensitive mobile path: %s", path)
		}
	}
}

func TestIsMobileSensitivePathAllowsOrdinaryFiles(t *testing.T) {
	for _, path := range []string{
		"docs/ci/MOBILE_SECRETS_POLICY.md",
		"android/README.md",
		"fastlane/Fastfile",
		"fastlane/.envrc",
		"apps/mobile/fastlane/.envrc",
		"ios/App.xcodeproj/project.pbxproj",
	} {
		if isMobileSensitivePath(path) {
			t.Fatalf("did not expect sensitive mobile path: %s", path)
		}
	}
}

func TestContainsGoogleServiceAccountPrivateKey(t *testing.T) {
	text := `{"type":"service_account","private_key":"` + "-----BEGIN " + `PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----\n"}`
	if !containsGoogleServiceAccountPrivateKey(text) {
		t.Fatal("expected service account private key to be detected")
	}
}

func TestWorkflowPolicyScanRejectsUnpinnedStepUses(t *testing.T) {
	repo := t.TempDir()
	workflowDir := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir failed: %v", err)
	}
	workflow := `name: verify
on: workflow_dispatch
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + `v4
`
	if err := os.WriteFile(filepath.Join(workflowDir, "verify.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow failed: %v", err)
	}

	t.Chdir(repo)
	err := runWorkflowPolicyScan()
	if err == nil {
		t.Fatal("expected unpinned step uses to fail")
	}
	if !strings.Contains(err.Error(), "non-SHA uses") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflowPolicyScanAcceptsPinnedStepUses(t *testing.T) {
	repo := t.TempDir()
	workflowDir := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir failed: %v", err)
	}
	workflow := `name: verify
on: workflow_dispatch
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5
`
	if err := os.WriteFile(filepath.Join(workflowDir, "verify.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow failed: %v", err)
	}

	t.Chdir(repo)
	if err := runWorkflowPolicyScan(); err != nil {
		t.Fatalf("expected pinned step uses to pass: %v", err)
	}
}

func TestWorkflowPolicyScanAcceptsUppercasePinnedStepUses(t *testing.T) {
	repo := t.TempDir()
	workflowDir := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir failed: %v", err)
	}
	workflow := `name: verify
on: workflow_dispatch
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34E114876B0B11C390A56381AD16EBD13914F8D5
`
	if err := os.WriteFile(filepath.Join(workflowDir, "verify.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow failed: %v", err)
	}

	t.Chdir(repo)
	if err := runWorkflowPolicyScan(); err != nil {
		t.Fatalf("expected uppercase pinned step uses to pass: %v", err)
	}
}
