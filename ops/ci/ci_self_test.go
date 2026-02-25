package ci_test

import (
	"os/exec"
	"strings"
	"testing"
)

func runCiSelf(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"./ci_self.sh"}, args...)...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestHelpListsRemoteCommands(t *testing.T) {
	out, err := runCiSelf(t, "help")
	if err != nil {
		t.Fatalf("help failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"up", "remote-register", "remote-run-focus", "remote-up"} {
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
