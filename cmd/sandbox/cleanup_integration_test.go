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
