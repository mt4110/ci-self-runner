package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestForwardSurvivesCancellationUntilCleanupFinishes(t *testing.T) {
	bin := t.TempDir()
	pidFile := filepath.Join(bin, "ssh.pid")
	t.Setenv("SSH_TEST_PID", pidFile)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	for name, script := range map[string]string{
		"ssh":    "#!/bin/sh\nprintf '%s' \"$$\" > \"$SSH_TEST_PID\"\nexec /bin/sleep 60\n",
		"docker": "#!/bin/sh\nprintf 'linux/x86_64\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, disconnect, err := dockerForward(ctx, "Host lima-ci-self-test\n  Hostname 127.0.0.1\n", "lima-ci-self-test")
	if err != nil {
		t.Fatal(err)
	}
	defer disconnect()
	var pid int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 {
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("SSH never started")
	}
	cancel()
	// A cleanup command must still be able to use the tunnel after cancellation.
	time.Sleep(30 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatal(fmt.Errorf("forward closed before cleanup: %w", err))
	}
}
