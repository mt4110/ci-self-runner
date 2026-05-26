package ci_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runVerifyFullWithEnv(t *testing.T, env []string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "./run_verify_full.sh")
	cmd.Dir = "."
	cmd.Env = mergeEnvOverrides(os.Environ(), env)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeFakeCommand(t *testing.T, dir string, name string, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake %s failed: %v", name, err)
	}
}

func TestRunVerifyFullDryRunSkipsDockerWhenUnavailable(t *testing.T) {
	binDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeFakeCommand(t, binDir, "docker", `#!/usr/bin/env sh
echo "docker should not be called during dry-run: $*" >&2
exit 99
`)

	out, err := runVerifyFullWithEnv(t, []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"OUT_DIR=" + outDir,
		"VERIFY_DRY_RUN=1",
		"GITHUB_ACTIONS=true",
		"GITHUB_RUN_ID=123456",
		"GITHUB_SHA=abc123",
		"GITHUB_REF_NAME=feature-branch",
	})
	if err != nil {
		t.Fatalf("expected dry-run to succeed without docker: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "SKIP: docker reason=dry_run") {
		t.Fatalf("expected docker skip message\noutput:\n%s", out)
	}
	if !strings.Contains(out, "STATUS: OK") {
		t.Fatalf("expected OK status line\noutput:\n%s", out)
	}

	statusPath := filepath.Join(outDir, "verify-full.status")
	body, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatalf("expected status file after dry-run: %v\noutput:\n%s", readErr, out)
	}
	status := string(body)
	for _, want := range []string{
		"status=OK",
		"mode=dry-run",
		"gha_sync=true",
		"github_run_id=123456",
		"github_sha=abc123",
		"github_ref=feature-branch",
		"source=run_verify_full",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %q\nstatus:\n%s\noutput:\n%s", want, status, out)
		}
	}

	logs, globErr := filepath.Glob(filepath.Join(outDir, "logs", "verify-full-*.log"))
	if globErr != nil || len(logs) != 1 {
		t.Fatalf("expected one dry-run log file, got %v err=%v\noutput:\n%s", logs, globErr, out)
	}
}

func TestRunVerifyFullWritesSpecificStatusWhenDockerCommandMissing(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")

	out, err := runVerifyFullWithEnv(t, []string{
		"PATH=/usr/bin:/bin",
		"OUT_DIR=" + outDir,
	})
	if err == nil {
		t.Fatalf("expected missing docker failure\noutput:\n%s", out)
	}
	if !strings.Contains(out, "docker command not found") {
		t.Fatalf("expected missing docker message\noutput:\n%s", out)
	}

	statusPath := filepath.Join(outDir, "verify-full.status")
	body, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatalf("expected status file after missing docker: %v\noutput:\n%s", readErr, out)
	}
	status := string(body)
	for _, want := range []string{
		"status=ERROR",
		"mode=full",
		"source=run_verify_full",
		"reason=docker_command_missing",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %q\nstatus:\n%s\noutput:\n%s", want, status, out)
		}
	}
}

func TestRunVerifyFullStartsColimaBeforeDockerRun(t *testing.T) {
	binDir := t.TempDir()
	stateDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	markerPath := filepath.Join(stateDir, "docker-ready")
	colimaLog := filepath.Join(stateDir, "colima.log")
	dockerLog := filepath.Join(stateDir, "docker-run.log")
	statusPath := filepath.Join(outDir, "verify-full.status")

	writeFakeCommand(t, binDir, "docker", `#!/usr/bin/env sh
case "$1" in
  info)
    test -f "$TEST_DOCKER_READY"
    ;;
  run)
    printf '%s\n' "$*" > "$TEST_DOCKER_LOG"
    mkdir -p "$(dirname "$TEST_STATUS_PATH")"
    {
      echo "status=OK"
      echo "source=fake-docker"
    } > "$TEST_STATUS_PATH"
    ;;
  *)
    echo "unexpected docker command: $*" >&2
    exit 99
    ;;
esac
`)
	writeFakeCommand(t, binDir, "colima", `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$TEST_COLIMA_LOG"
case "$1" in
  status)
    exit 1
    ;;
  start)
    touch "$TEST_DOCKER_READY"
    ;;
  *)
    echo "unexpected colima command: $*" >&2
    exit 98
    ;;
esac
`)

	out, err := runVerifyFullWithEnv(t, []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"OUT_DIR=" + outDir,
		"TEST_DOCKER_READY=" + markerPath,
		"TEST_COLIMA_LOG=" + colimaLog,
		"TEST_DOCKER_LOG=" + dockerLog,
		"TEST_STATUS_PATH=" + statusPath,
	})
	if err != nil {
		t.Fatalf("expected colima recovery to succeed: %v\noutput:\n%s", err, out)
	}

	colimaBody, readErr := os.ReadFile(colimaLog)
	if readErr != nil {
		t.Fatalf("expected colima log: %v\noutput:\n%s", readErr, out)
	}
	if !strings.Contains(string(colimaBody), "start") {
		t.Fatalf("expected colima start to be called\nlog:\n%s\noutput:\n%s", string(colimaBody), out)
	}
	dockerBody, readErr := os.ReadFile(dockerLog)
	if readErr != nil {
		t.Fatalf("expected docker run log: %v\noutput:\n%s", readErr, out)
	}
	if !strings.Contains(string(dockerBody), "/usr/local/bin/verify-full") {
		t.Fatalf("expected verify-full container command\nlog:\n%s\noutput:\n%s", string(dockerBody), out)
	}
}

func TestRunVerifyFullWritesStatusWhenDockerRunFailsWithoutStatus(t *testing.T) {
	binDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeFakeCommand(t, binDir, "docker", `#!/usr/bin/env sh
case "$1" in
  info)
    exit 0
    ;;
  run)
    echo "container failed before status" >&2
    exit 42
    ;;
  *)
    echo "unexpected docker command: $*" >&2
    exit 99
    ;;
esac
`)

	out, err := runVerifyFullWithEnv(t, []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"OUT_DIR=" + outDir,
	})
	if err == nil {
		t.Fatalf("expected docker run failure\noutput:\n%s", out)
	}

	statusPath := filepath.Join(outDir, "verify-full.status")
	body, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatalf("expected status file after docker run failure: %v\noutput:\n%s", readErr, out)
	}
	status := string(body)
	for _, want := range []string{
		"status=ERROR",
		"source=run_verify_full",
		"reason=docker_run_failed",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %q\nstatus:\n%s\noutput:\n%s", want, status, out)
		}
	}
}
