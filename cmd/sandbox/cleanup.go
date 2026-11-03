package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// act 0.2.88 reuses deterministic job names and can remove a previous failed
// container at startup. Preserve both owned evidence and unrelated act jobs by
// refusing to start while an act-named container remains on this engine.
func guardActContainers(ctx context.Context, host string) error {
	rows, err := capture(ctx, "", "docker", "--host", host, "ps", "--all", "--no-trunc", "--format", "{{.ID}}\t{{.Names}}")
	if err != nil {
		return err
	}
	var retained []string
	for _, row := range strings.Split(rows, "\n") {
		id, name, ok := strings.Cut(row, "\t")
		if ok && strings.HasPrefix(name, "act-") {
			retained = append(retained, id+" ("+name+")")
		}
	}
	if len(retained) > 0 {
		return fmt.Errorf("existing act containers must be inspected before another workflow: %s; act may delete or reuse them; no containers were changed", strings.Join(retained, ", "))
	}
	return nil
}

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
