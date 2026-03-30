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

func TestRemoteRegisterFallsBackToHomeLocalBin(t *testing.T) {
	tmp := t.TempDir()
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
	}

	logPath := filepath.Join(tmp, "ssh.log")
	sshPath := filepath.Join(tmp, "ssh")
	sshScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "$*" >> %q
if [[ "$*" == *"BatchMode=yes"* ]]; then
  exit 0
fi
exit 42
`, logPath)
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"remote-register",
		"--host",
		"mini-user@192.168.1.9",
		"-i",
		identityPath,
		"--project-dir",
		"~/dev/zt-gateway",
		"--repo",
		"mt4110/zt-gateway",
		"--skip-workflow",
	)
	if err == nil {
		t.Fatalf("expected failure from fake ssh, got success\noutput:\n%s", out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read ssh log: %v", readErr)
	}
	if !strings.Contains(string(logBody), "$HOME/.local/bin/$remote_cli") {
		t.Fatalf("expected remote register to fall back to ~/.local/bin/ci-self\nlog:\n%s", string(logBody))
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
	if !strings.Contains(content, "CI_SELF_REMOTE_IDENTITY=") {
		t.Fatalf("missing remote identity placeholder in config\ncontent:\n%s", content)
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

func TestRemoteCIRejectsWrongDefaultLocalDir(t *testing.T) {
	tmp := t.TempDir()
	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		nil,
		"remote-ci",
		"--host",
		"mini-user@192.168.1.9",
		"--repo",
		"mt4110/veil-rs",
		"--skip-bootstrap",
	)
	if err == nil {
		t.Fatalf("expected remote-ci to reject mismatched default local dir\noutput:\n%s", out)
	}
	if !strings.Contains(out, "ERROR: default local-dir appears to be the wrong project") {
		t.Fatalf("expected mismatched-local-dir error\noutput:\n%s", out)
	}
	if !strings.Contains(out, "pass --local-dir <path>") {
		t.Fatalf("expected local-dir hint\noutput:\n%s", out)
	}
}

func TestRemoteCIRunsSyncVerifyAndFetch(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
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
		"-i",
		identityPath,
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

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read tool log: %v", readErr)
	}
	logText := string(logBody)
	if !strings.Contains(logText, "ssh -i "+identityPath) {
		t.Fatalf("expected ssh to receive identity file\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "rsync -az --delete -e ssh -i "+identityPath+" --human-readable --info=progress2") &&
		!strings.Contains(logText, "rsync -az --delete --human-readable --info=progress2 -e ssh -i "+identityPath) {
		t.Fatalf("expected rsync sync to receive identity file\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude target/") {
		t.Fatalf("expected rsync sync to exclude target dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude dist/") {
		t.Fatalf("expected rsync sync to exclude dist dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude node_modules/") {
		t.Fatalf("expected rsync sync to exclude node_modules dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude .venv/") {
		t.Fatalf("expected rsync sync to exclude python virtualenv dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude coverage/") {
		t.Fatalf("expected rsync sync to exclude coverage dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude .next/") {
		t.Fatalf("expected rsync sync to exclude next build dir\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "--exclude .git/") {
		t.Fatalf("expected rsync sync to exclude git dir by default\nlog:\n%s", logText)
	}
	if !strings.Contains(out, "cmd=remote_verify_wrapper") {
		t.Fatalf("expected remote verify wrapper invocation\noutput:\n%s", out)
	}
	if !strings.Contains(logText, "sh\\ -s") && !strings.Contains(logText, "sh -s") {
		t.Fatalf("expected remote verify wrapper to stream script over ssh\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "rsync -a -e ssh -i "+identityPath) {
		t.Fatalf("expected rsync fetch to receive identity file\nlog:\n%s", logText)
	}

	statusPath := filepath.Join(localDir, "out", "remote", "mini-user-192.168.1.9", "verify-full.status")
	if _, statErr := os.Stat(statusPath); statErr != nil {
		t.Fatalf("expected fetched status file at %s: %v", statusPath, statErr)
	}
}

func TestRemoteCIWithTildeProjectDirUsesRemoteHome(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "veil-rs")
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
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
		"-i",
		identityPath,
		"--local-dir",
		localDir,
		"--project-dir",
		"~/_workspace/veil-rs",
		"--skip-bootstrap",
	)
	if err != nil {
		t.Fatalf("remote-ci with tilde project dir failed: %v\noutput:\n%s", err, out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read tool log: %v", readErr)
	}
	logText := string(logBody)
	if strings.Contains(logText, "$HOME/~/_workspace/veil-rs") {
		t.Fatalf("tilde path was expanded incorrectly\nlog:\n%s", logText)
	}
	if strings.Contains(logText, "\\$HOME/_workspace/veil-rs") {
		t.Fatalf("tilde path should expand on remote, not remain escaped\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "_workspace/veil-rs") {
		t.Fatalf("expected remote command to target remote project dir\nlog:\n%s", logText)
	}
}

func TestRemoteCISyncGitDirOptIn(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
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
		"-i",
		identityPath,
		"--local-dir",
		localDir,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-bootstrap",
		"--sync-git-dir",
	)
	if err != nil {
		t.Fatalf("remote-ci failed: %v\noutput:\n%s", err, out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read tool log: %v", readErr)
	}
	logText := string(logBody)
	if strings.Contains(logText, "--exclude .git/") {
		t.Fatalf("expected sync-git-dir to keep .git in sync set\nlog:\n%s", logText)
	}
}

func TestRemoteCIFallsBackForLegacyRsync(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
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
if [[ "$*" == *"--info=progress2 --version"* ]]; then
  exit 1
fi
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
		"-i",
		identityPath,
		"--local-dir",
		localDir,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-bootstrap",
	)
	if err != nil {
		t.Fatalf("remote-ci failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "falling back to -h --progress") {
		t.Fatalf("expected fallback warning in output\noutput:\n%s", out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read tool log: %v", readErr)
	}
	logText := string(logBody)
	if !strings.Contains(logText, "rsync -az --delete -e ssh -i "+identityPath+" -h --progress") &&
		!strings.Contains(logText, "rsync -az --delete -h --progress -e ssh -i "+identityPath) {
		t.Fatalf("expected legacy rsync fallback flags\nlog:\n%s", logText)
	}
}

func TestRemoteCIFetchFallsBackToSSH(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "repo")
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	fixtureDir := filepath.Join(tmp, "remote-fixture")
	fixtureLogsDir := filepath.Join(fixtureDir, "logs")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local repo failed: %v", err)
	}
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
	}
	if err := os.MkdirAll(fixtureLogsDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture logs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "verify-full.status"), []byte("status=OK\n"), 0o644); err != nil {
		t.Fatalf("write fixture status failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureLogsDir, "verify.log"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write fixture log failed: %v", err)
	}

	logPath := filepath.Join(tmp, "tool.log")
	sshPath := filepath.Join(tmp, "ssh")
	sshScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "ssh $*" >> %q
if [[ "$*" == *"BatchMode=yes"* ]]; then
  exit 0
fi
if [[ "$*" == *"verify-full.status"* && "$*" == *"cat"* ]]; then
  cat %q
  exit 0
fi
if [[ "$*" == *"tar"* && "$*" == *"logs"* ]]; then
  tar -cf - -C %q logs
  exit 0
fi
if [[ "$*" == *"mkdir -p"* || "$*" == *"sh"* || "$*" == *"status_file="* || "$*" == *"logs_dir="* ]]; then
  exit 0
fi
exit 1
`, logPath, filepath.Join(fixtureDir, "verify-full.status"), fixtureDir)
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	rsyncPath := filepath.Join(tmp, "rsync")
	rsyncScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "rsync $*" >> %q
src="${@: -2:1}"
if printf '%%s' "$src" | grep -q '/out/verify-full.status$'; then
  exit 23
fi
if printf '%%s' "$src" | grep -q '/out/logs/$'; then
  exit 23
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
		"-i",
		identityPath,
		"--local-dir",
		localDir,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-bootstrap",
	)
	if err != nil {
		t.Fatalf("remote-ci failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "source=ssh_fallback") {
		t.Fatalf("expected ssh fallback fetch output\noutput:\n%s", out)
	}

	statusPath := filepath.Join(localDir, "out", "remote", "mini-user-192.168.1.9", "verify-full.status")
	if content, readErr := os.ReadFile(statusPath); readErr != nil {
		t.Fatalf("expected fetched status file via ssh fallback: %v", readErr)
	} else if !strings.Contains(string(content), "status=OK") {
		t.Fatalf("unexpected status file content: %s", string(content))
	}

	logFilePath := filepath.Join(localDir, "out", "remote", "mini-user-192.168.1.9", "logs", "verify.log")
	if content, readErr := os.ReadFile(logFilePath); readErr != nil {
		t.Fatalf("expected fetched log via ssh fallback: %v", readErr)
	} else if strings.TrimSpace(string(content)) != "ok" {
		t.Fatalf("unexpected fetched log content: %s", string(content))
	}
}

func TestRemoteUpAcceptsIdentity(t *testing.T) {
	tmp := t.TempDir()
	identityPath := filepath.Join(tmp, "id_ed25519_for_mac_mini")
	if err := os.WriteFile(identityPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
	}

	logPath := filepath.Join(tmp, "ssh.log")
	sshPath := filepath.Join(tmp, "ssh")
	sshScript := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" >> %q\nexit 42\n", logPath)
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh failed: %v", err)
	}

	out, err := runCiSelfInDirEnv(
		t,
		tmp,
		[]string{"PATH=" + tmp + ":" + os.Getenv("PATH")},
		"remote-up",
		"--host",
		"mini-user@192.168.1.9",
		"-i",
		identityPath,
		"--project-dir",
		"~/dev/zt-gateway",
		"--skip-workflow",
		"--ref",
		"main",
	)
	if err == nil {
		t.Fatalf("expected failure from fake ssh, got success\noutput:\n%s", out)
	}
	if strings.Contains(out, "unknown option for remote-up: -i") {
		t.Fatalf("identity option was not parsed\noutput:\n%s", out)
	}

	logBody, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("failed to read ssh log: %v", readErr)
	}
	if !strings.Contains(string(logBody), "-i "+identityPath) {
		t.Fatalf("expected ssh call to include identity file\nlog:\n%s", string(logBody))
	}
}
