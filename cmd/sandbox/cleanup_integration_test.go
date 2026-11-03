package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Opt-in: creates and stops a labelled container, retains it for inspection.
// No container, volume, image or VM is deleted by this test.
func TestStopRunContainersIntegration(t *testing.T) {
	host := os.Getenv("CI_SELF_TEST_DOCKER_HOST")
	image := os.Getenv("CI_SELF_TEST_DOCKER_IMAGE")
	if host == "" || image == "" {
		t.Skip("set explicit Docker endpoint and existing image for integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	runID := fmt.Sprintf("run-cleanup-test-%d", time.Now().UnixNano())
	c := exec.CommandContext(ctx, "docker", "--host", host, "run", "--detach", "--platform", "linux/amd64", "--label", "ci-self.run="+runID, image, "/bin/sleep", "300")
	c.Env = dockerEndpointEnv(host)
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("create fixture: %v: %s", err, b)
	}
	id := strings.TrimSpace(string(b))
	t.Cleanup(func() { _, _ = stopRunContainers(context.Background(), host, runID, io.Discard) })
	cancelled, stop := context.WithCancel(ctx)
	stop()
	stopped, err := stopRunContainers(cancelled, host, runID, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 1 || stopped[0] != id {
		t.Fatalf("stopped %v, wanted only %s", stopped, id)
	}
	state, err := capture(ctx, "", "docker", "--host", host, "inspect", "--format", "{{.State.Status}}", id)
	if err != nil || state != "exited" {
		t.Fatalf("retained container state %q: %v", state, err)
	}
	t.Logf("retained stopped fixture container=%s run_label=ci-self.run=%s", id, runID)
}

func TestActGuardIntegration(t *testing.T) {
	host, image := os.Getenv("CI_SELF_TEST_DOCKER_HOST"), os.Getenv("CI_SELF_TEST_DOCKER_IMAGE")
	if host == "" || image == "" {
		t.Skip("set explicit Docker endpoint and existing image for integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	runID := fmt.Sprintf("run-guard-test-%d", time.Now().UnixNano())
	c := exec.CommandContext(ctx, "docker", "--host", host, "run", "--detach", "--platform", "linux/amd64", "--name", "act-"+runID, "--label", "ci-self.run="+runID, image, "/bin/sleep", "300")
	c.Env = dockerEndpointEnv(host)
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("create guard fixture: %v: %s", err, b)
	}
	id := strings.TrimSpace(string(b))
	t.Cleanup(func() {
		if _, err := stopRunContainers(context.Background(), host, runID, io.Discard); err != nil {
			t.Error(err)
		}
		// This fixture has no act job volumes. Retain it under a non-act name so
		// it does not intentionally block subsequent real workflow executions.
		cleanupCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if _, err := capture(cleanupCtx, "", "docker", "--host", host, "rename", id, "ci-self-retained-"+runID); err != nil {
			t.Error(err)
		}
	})
	if err := guardActContainers(ctx, host); err == nil || !strings.Contains(err.Error(), id) {
		t.Fatalf("guard did not report existing fixture %s: %v", id, err)
	}
	state, err := capture(ctx, "", "docker", "--host", host, "inspect", "--format", "{{.State.Running}}", id)
	if err != nil || state != "true" {
		t.Fatalf("guard changed fixture state: %q %v", state, err)
	}
	t.Logf("guard rejected existing act container without stopping it: %s", id)
}

func TestContainerInitReapsOrphansIntegration(t *testing.T) {
	host, image := os.Getenv("CI_SELF_TEST_DOCKER_HOST"), os.Getenv("CI_SELF_TEST_DOCKER_IMAGE")
	if host == "" || image == "" {
		t.Skip("set explicit Docker endpoint and existing image for integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("ci-self-reaper-test-%d", time.Now().UnixNano())
	script := `/bin/sh -c '/bin/sleep 1 & echo $! > /tmp/child-pid'
/bin/sleep 3
pid=$(/bin/cat /tmp/child-pid)
if kill -0 "$pid" 2>/dev/null; then echo 'orphan remains alive or zombie' >&2; exit 1; fi
echo orphan_reaped=OK
`
	c := exec.CommandContext(ctx, "docker", "--host", host, "run", "--init", "--name", name, image, "/bin/sh", "-ec", script)
	c.Env = dockerEndpointEnv(host)
	b, err := c.CombinedOutput()
	if err != nil || !strings.Contains(string(b), "orphan_reaped=OK") {
		t.Fatalf("init failed to reap orphan: %v: %s", err, b)
	}
	init, err := capture(ctx, "", "docker", "--host", host, "inspect", "--format", "{{.HostConfig.Init}}", name)
	if err != nil || init != "true" {
		t.Fatalf("init not enabled: %s %v", init, err)
	}
	t.Logf("orphan reaped; retained stopped fixture=%s", name)
}
