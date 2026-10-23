package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// connectDocker never restarts an engine. If the default Colima Unix forwarding
// is stale but its VM is alive, use a process-scoped SSH forward to that same VM.
func connectDocker(ctx context.Context, vm string) (host, platform string, disconnect func(), err error) {
	disconnect = func() {}
	if vm != "" {
		path, readErr := capture(ctx, "", "limactl", "list", "--format", "{{.SSHConfigFile}}", vm)
		if readErr != nil || path == "" {
			err = fmt.Errorf("Lima VM %s not found; start it first", vm)
			return
		}
		config, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
			return
		}
		return dockerForward(ctx, string(config), "lima-"+vm)
	}
	host = os.Getenv("DOCKER_HOST")
	explicit := host != ""
	if host == "" {
		host, err = capture(ctx, "", "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
		if err != nil {
			return
		}
	}
	probe := func(endpoint string) (string, error) {
		return capture(ctx, "", "docker", "--host", endpoint, "info", "--format", "{{.OSType}}/{{.Architecture}}")
	}
	platform, err = probe(host)
	if err == nil {
		return
	}
	failure := errors.New("Docker daemon unreachable; start Docker/Colima or set DOCKER_HOST (no automatic service restart)")
	if explicit || !strings.HasPrefix(host, "unix://") || !strings.HasSuffix(host, "/.colima/default/docker.sock") {
		err = failure
		return
	}
	config, configErr := capture(ctx, "", "colima", "ssh-config")
	if configErr != nil || !strings.Contains(config, "Hostname 127.0.0.1") {
		err = failure
		return
	}
	return dockerForward(ctx, config, "colima")
}

func dockerForward(ctx context.Context, config, alias string) (host, platform string, disconnect func(), err error) {
	disconnect = func() {}
	if !strings.Contains(config, "Hostname 127.0.0.1") {
		err = errors.New("VM SSH must connect to localhost")
		return
	}
	failure := fmt.Errorf("Docker unreachable inside %s; check the VM and Docker service", alias)
	probe := func(endpoint string) (string, error) {
		return capture(ctx, "", "docker", "--host", endpoint, "info", "--format", "{{.OSType}}/{{.Architecture}}")
	}
	// Keep the Unix socket path short on macOS, independently of TMPDIR.
	dir, dirErr := os.MkdirTemp("/tmp", "ci-self-docker-")
	if dirErr != nil {
		err = dirErr
		return
	}
	configPath := filepath.Join(dir, "ssh.config")
	if err = os.WriteFile(configPath, []byte(config+"\n"), 0o600); err != nil {
		return
	}
	socket := filepath.Join(dir, "docker.sock")
	// Keep forwarding alive while act handles cancellation and cleans up.
	// The caller disconnects after the child has exited; startup still observes ctx.
	tunnelCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	ssh := exec.CommandContext(tunnelCtx, "ssh", "-F", configPath, "-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ExitOnForwardFailure=yes", "-N", "-L", socket+":/var/run/docker.sock", alias)
	ssh.Stderr = os.Stderr
	if err = ssh.Start(); err != nil {
		stop()
		return
	}
	done := make(chan error, 1)
	go func() { done <- ssh.Wait() }()
	disconnect = func() { stop(); <-done }
	host = "unix://" + socket
	for i := 0; i < 30; i++ {
		platform, err = probe(host)
		if err == nil {
			fmt.Printf("OK: using temporary SSH forwarding to VM=%s; no service restart\n", alias)
			return
		}
		select {
		case <-ctx.Done():
			stop()
			<-done
			disconnect = func() {}
			err = ctx.Err()
			return
		case <-done:
			stop()
			disconnect = func() {}
			err = failure
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	disconnect()
	disconnect = func() {}
	err = failure
	return
}

// An explicit engine endpoint must not be overridden by a host Docker context.
func dockerEndpointEnv(host string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "DOCKER_CONTEXT=") || strings.HasPrefix(kv, "DOCKER_HOST=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "DOCKER_HOST="+host)
}
