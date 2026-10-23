package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Stop only this invocation's labelled leftovers. Never remove containers,
// volumes, images, run evidence, or the VM. Preserve forwarding during cleanup.
func stopRunContainers(ctx context.Context, host, runID string, output io.Writer) ([]string, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	ids, err := capture(cleanupCtx, "", "docker", "--host", host, "ps", "--no-trunc", "--filter", "label=ci-self.run="+runID, "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}
	var stopped []string
	for _, id := range strings.Fields(ids) {
		c := exec.CommandContext(cleanupCtx, "docker", "--host", host, "stop", "--timeout", "10", id)
		c.Env = dockerEndpointEnv(host)
		c.Stdout, c.Stderr = output, output
		if err := c.Run(); err != nil {
			return stopped, fmt.Errorf("stop owned container %s: %w", id, err)
		}
		stopped = append(stopped, id)
	}
	return stopped, nil
}
