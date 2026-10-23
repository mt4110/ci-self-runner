package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println("Usage: ci-self vm start|status|stop|shell [--name ci-self-x64] [-- command args...]\nUse nix develop .#vm for QEMU. This command never deletes a VM.")
		return nil
	}
	action := args[0]
	f := flag.NewFlagSet("ci-self vm "+action, flag.ContinueOnError)
	name := f.String("name", "ci-self-x64", "dedicated Lima VM name")
	config := f.String("config", "ci/vm/ubuntu-x64.yaml", "VM template, used only for a new instance")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if !regexp.MustCompile(`^ci-self-[a-z0-9-]+$`).MatchString(*name) {
		return errors.New("VM name must start with ci-self- and contain lowercase letters, digits or hyphens")
	}
	if action != "shell" && len(f.Args()) != 0 {
		return errors.New("unexpected positional arguments")
	}
	cli := os.Getenv("CI_SELF_LIMACTL")
	if cli == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		// A matching official Lima distribution can be installed here without
		// upgrading or overwriting a Homebrew installation used by other VMs.
		candidate := filepath.Join(cache, "ci-self-runner", "lima-2.2.0", "bin", "limactl")
		// macOS user cache differs from ~/.cache; accept the prepared tools path.
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		for _, p := range []string{candidate, filepath.Join(home, ".cache", "ci-self-runner", "lima-2.2.0", "bin", "limactl")} {
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
				cli = p
				break
			}
		}
		if cli == "" {
			var err error
			cli, err = exec.LookPath("limactl")
			if err != nil {
				return errors.New("Lima is required")
			}
		}
	}
	var command []string
	switch action {
	case "status":
		command = []string{"list", *name}
	case "stop":
		command = []string{"stop", *name}
	case "shell":
		command = append([]string{"shell", "--workdir=/home/ci", *name, "--"}, f.Args()...)
	case "start":
		if _, err := exec.LookPath("qemu-system-x86_64"); err != nil {
			return errors.New("qemu-system-x86_64 is required; enter nix develop .#vm")
		}
		probe := exec.CommandContext(ctx, cli, "list", "--format", "{{.Name}}", *name)
		out, err := probe.Output()
		if err != nil {
			return fmt.Errorf("inspect VM: %w", err)
		}
		if len(out) > 0 {
			command = []string{"start", *name, "--timeout=30m", "--tty=false"}
		} else {
			if _, err := os.Stat(*config); err != nil {
				return err
			}
			command = []string{"start", "--name=" + *name, "--timeout=30m", "--tty=false", *config}
		}
	default:
		return fmt.Errorf("unknown VM command: %s", action)
	}
	started := time.Now()
	c := exec.CommandContext(ctx, cli, command...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Cancel = func() error { return c.Process.Signal(os.Interrupt) }
	c.WaitDelay = 10 * time.Second
	err := c.Run()
	fmt.Fprintf(os.Stderr, "VM: action=%s name=%s elapsed_sec=%.3f\n", action, *name, time.Since(started).Seconds())
	return err
}
