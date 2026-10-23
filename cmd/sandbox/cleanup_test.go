package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupStopsOnlyRunLabelAfterCancellationWithoutDeleting(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	t.Setenv("CLEANUP_TEST_LOG", log)
	t.Setenv("DOCKER_CONTEXT", "unrelated-engine")
	t.Setenv("DOCKER_HOST", "unix:///unrelated.sock")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	script := "#!/bin/sh\n[ -z \"$DOCKER_CONTEXT\" ] && [ \"$DOCKER_HOST\" = unix:///owned.sock ] || exit 32\nprintf '%s\\n' \"$*\" >> \"$CLEANUP_TEST_LOG\"\ncase \"$3\" in ps) printf 'abc123\\n';; stop) exit 0;; *) exit 31;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopped, err := stopRunContainers(ctx, "unix:///owned.sock", "run-owned", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 1 || stopped[0] != "abc123" {
		t.Fatalf("stopped=%v", stopped)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(b)
	for _, want := range []string{"--filter label=ci-self.run=run-owned", "stop --timeout 10 abc123"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %s: %s", want, calls)
		}
	}
	for _, bad := range []string{" rm ", "prune", "volume", "system"} {
		if strings.Contains(calls, bad) {
			t.Fatalf("destructive cleanup: %s", calls)
		}
	}
}
