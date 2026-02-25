package ci_test

import (
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

func TestHelpListsRemoteCommands(t *testing.T) {
	out, err := runCiSelf(t, "help")
	if err != nil {
		t.Fatalf("help failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"up", "focus", "doctor", "config-init", "remote-register", "remote-run-focus", "remote-up"} {
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
