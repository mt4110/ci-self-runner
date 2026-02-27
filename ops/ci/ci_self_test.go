package ci_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runCiSelfInDirEnv(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	scriptPath, err := filepath.Abs("./ci_self.sh")
	if err != nil {
		t.Fatalf("failed to resolve ci_self.sh path: %v", err)
	}
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	if dir == "" {
		cmd.Dir = "."
	} else {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, runErr := cmd.CombinedOutput()
	return string(out), runErr
}

func runCiSelf(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runCiSelfInDirEnv(t, ".", nil, args...)
}

func writeFakeGH(t *testing.T, dir string, body string) {
	t.Helper()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte(body), 0o755); err != nil {
		t.Fatalf("failed to write fake gh: %v", err)
	}
}

func TestHelpListsRemoteCommands(t *testing.T) {
	out, err := runCiSelf(t, "help")
	if err != nil {
		t.Fatalf("help failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"up", "focus", "doctor", "config-init", "remote-ci", "remote-register", "remote-run-focus", "remote-up"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q\noutput:\n%s", want, out)
		}
	}
}

func TestRemoteUpRequiresHost(t *testing.T) {
	out, err := runCiSelf(t, "remote-up")
	if err == nil {
		t.Fatalf("expected failure without --host, got success\noutput:\n%s", out)
	}
	if !strings.Contains(out, "ERROR: --host is required") {
		t.Fatalf("unexpected error output\noutput:\n%s", out)
	}
}

func TestRemoteRegisterRequiresHost(t *testing.T) {
	out, err := runCiSelf(t, "remote-register")
	if err == nil {
		t.Fatalf("expected failure without --host, got success\noutput:\n%s", out)
	}
	if !strings.Contains(out, "ERROR: --host is required") {
		t.Fatalf("unexpected error output\noutput:\n%s", out)
	}
}

func TestUpHelp(t *testing.T) {
	out, err := runCiSelf(t, "up", "--help")
	if err != nil {
		t.Fatalf("up --help failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Usage: ci-self up") {
		t.Fatalf("up help output missing usage\noutput:\n%s", out)
	}
}

func TestFocusHelp(t *testing.T) {
	out, err := runCiSelf(t, "focus", "--help")
	if err != nil {
		t.Fatalf("focus --help failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Usage: ci-self focus") {
		t.Fatalf("focus help output missing usage\noutput:\n%s", out)
	}
}

func TestDoctorHelp(t *testing.T) {
	out, err := runCiSelf(t, "doctor", "--help")
	if err != nil {
		t.Fatalf("doctor --help failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Usage: ci-self doctor") {
		t.Fatalf("doctor help output missing usage\noutput:\n%s", out)
	}
}

func TestConfigInitWritesFile(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, ".ci-self.env")
	out, err := runCiSelf(t, "config-init", "--path", cfg, "--repo", "mt4110/maakie-brainlab", "--project-dir", tmp, "--force")
	if err != nil {
		t.Fatalf("config-init failed: %v\noutput:\n%s", err, out)
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "CI_SELF_REPO=mt4110/maakie-brainlab") {
		t.Fatalf("missing repo in config\ncontent:\n%s", content)
	}
	if !strings.Contains(content, "CI_SELF_PROJECT_DIR="+tmp) {
		t.Fatalf("missing project dir in config\ncontent:\n%s", content)
	}
}

func TestRemoteUpUsesConfigHost(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, ".ci-self.env")
	cfgContent := "CI_SELF_REMOTE_HOST=config-host\nCI_SELF_REPO=mt4110/maakie-brainlab\nCI_SELF_REMOTE_PROJECT_DIR=~/dev/maakie-brainlab\n"
	if err := os.WriteFile(cfg, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin failed: %v", err)
	}
	sshPath := filepath.Join(binDir, "ssh")
	sshScript := "#!/usr/bin/env bash\necho \"FAKE_SSH $*\"\nexit 42\n"
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + binDir + ":" + os.Getenv("PATH")},
		"remote-up",
		"--skip-workflow",
		"--ref",
		"main",
	)
	if err == nil {
		t.Fatalf("expected failure from fake ssh, got success\noutput:\n%s", out)
	}
	if strings.Contains(out, "ERROR: --host is required") {
		t.Fatalf("config host was not applied\noutput:\n%s", out)
	}
	if !strings.Contains(out, "OK: ssh host=config-host") {
		t.Fatalf("expected config host in output\noutput:\n%s", out)
	}
}

func TestRunWatchResolvesVerifyWorkflowID(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "gh.log")
	fakeGH := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "$*" >> %q
if [[ "${1:-}" == "api" && "${2:-}" == "repos/mt4110/zt-gateway/actions/workflows" ]]; then
  printf '42\t.github/workflows/verify.yaml\tverify\n'
  exit 0
fi
if [[ "${1:-}" == "workflow" && "${2:-}" == "run" ]]; then
  exit 0
fi
if [[ "${1:-}" == "run" && "${2:-}" == "list" ]]; then
  echo "98765"
  exit 0
fi
if [[ "${1:-}" == "run" && "${2:-}" == "watch" ]]; then
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`, logPath)
	writeFakeGH(t, tmp, fakeGH)

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"run-watch",
		"--repo",
		"mt4110/zt-gateway",
		"--ref",
		"main",
	)
	if err != nil {
		t.Fatalf("run-watch failed: %v\noutput:\n%s", err, out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read gh log: %v", readErr)
	}
	logText := string(logBody)
	if !strings.Contains(logText, "workflow run 42 --ref main -R mt4110/zt-gateway") {
		t.Fatalf("expected workflow run to use resolved id\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "run list --workflow 42 -R mt4110/zt-gateway") {
		t.Fatalf("expected run list to use resolved id\nlog:\n%s", logText)
	}
}

func TestRunWatchMissingVerifyWorkflowShowsHint(t *testing.T) {
	tmp := t.TempDir()
	fakeGH := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "api" && "${2:-}" == "repos/mt4110/zt-gateway/actions/workflows" ]]; then
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 1
`
	writeFakeGH(t, tmp, fakeGH)

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"run-watch",
		"--repo",
		"mt4110/zt-gateway",
		"--ref",
		"main",
	)
	if err == nil {
		t.Fatalf("expected run-watch failure when verify workflow is missing\noutput:\n%s", out)
	}
	if !strings.Contains(out, "ERROR: verify workflow not found in remote repo (mt4110/zt-gateway)") {
		t.Fatalf("expected missing-workflow error\noutput:\n%s", out)
	}
	if !strings.Contains(out, "scaffold_verify_workflow.sh") {
		t.Fatalf("expected scaffold hint in output\noutput:\n%s", out)
	}
}

func TestRemoteCIRequiresKeyAuth(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}

	sshPath := filepath.Join(tmp, "ssh")
	sshScript := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"BatchMode=yes"* ]]; then
  exit 1
fi
exit 0
`
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	rsyncPath := filepath.Join(tmp, "rsync")
	rsyncScript := "#!/usr/bin/env bash\nexit 0\n"
	if err := os.WriteFile(rsyncPath, []byte(rsyncScript), 0o755); err != nil {
		t.Fatalf("write fake rsync failed: %v", err)
	}

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"remote-ci",
		"--host",
		"mini-user@192.168.1.9",
		"--local-dir",
		localDir,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-bootstrap",
	)
	if err == nil {
		t.Fatalf("expected key auth failure\noutput:\n%s", out)
	}
	if !strings.Contains(out, "ERROR: ssh key-based auth failed") {
		t.Fatalf("expected key auth error output\noutput:\n%s", out)
	}
}

func TestRemoteCIRunsSyncVerifyAndFetch(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(localDir, "ops", "ci"), 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "ops", "ci", "run_verify_full.sh"), []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write local verify script failed: %v", err)
	}

	logPath := filepath.Join(tmp, "tool.log")
	sshPath := filepath.Join(tmp, "ssh")
	sshScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "ssh $*" >> %q
if [[ "$*" == *"BatchMode=yes"* ]]; then
  exit 0
fi
exit 0
`, logPath)
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	rsyncPath := filepath.Join(tmp, "rsync")
	rsyncScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "rsync $*" >> %q
src="${@: -2:1}"
dst="${@: -1}"
if printf '%%s' "$src" | grep -q '/out/verify-full.status$'; then
  mkdir -p "$dst"
  cat > "${dst%%/}/verify-full.status" <<'EOF'
status=OK
EOF
  exit 0
fi
if printf '%%s' "$src" | grep -q '/out/logs/$'; then
  mkdir -p "${dst%%/}"
  echo "ok" > "${dst%%/}/verify.log"
  exit 0
fi
exit 0
`, logPath)
	if err := os.WriteFile(rsyncPath, []byte(rsyncScript), 0o755); err != nil {
		t.Fatalf("write fake rsync failed: %v", err)
	}

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"remote-ci",
		"--host",
		"mini-user@192.168.1.9",
		"--local-dir",
		localDir,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-bootstrap",
	)
	if err != nil {
		t.Fatalf("remote-ci failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "OK: remote-ci result status=OK") {
		t.Fatalf("expected success status output\noutput:\n%s", out)
	}

	statusPath := filepath.Join(localDir, "out", "remote", "mini-user-192.168.1.9", "verify-full.status")
	if _, statErr := os.Stat(statusPath); statErr != nil {
		t.Fatalf("expected fetched status file at %s: %v", statusPath, statErr)
	}
}
