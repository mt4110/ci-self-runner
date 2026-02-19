package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			cfg := loadConfig()
			_ = writeStatus(cfg, "ERROR", fmt.Sprintf("panic=%v", r))
			fmt.Printf("ERROR: verify-lite panic=%v\n", r)
			fmt.Println("STATUS: ERROR")
		}
	}()

	cfg := loadConfig()
	if err := run(cfg); err != nil {
		_ = writeStatus(cfg, "ERROR", err.Error())
		fmt.Printf("ERROR: verify-lite %s\n", err.Error())
		fmt.Println("STATUS: ERROR")
		return
	}
	_ = writeStatus(cfg, "OK", "")
	fmt.Println("OK: verify-lite completed")
	fmt.Println("STATUS: OK")
}

type config struct {
	repoDir    string
	outDir     string
	stamp      string
	timeoutSec int
}

func loadConfig() config {
	timeoutSec, err := envOrInt("VERIFY_LITE_TIMEOUT_SEC", 600)
	if err != nil {
		timeoutSec = 600
	}
	return config{
		repoDir:    envOr("REPO_DIR", "."),
		outDir:     envOr("OUT_DIR", "out"),
		stamp:      time.Now().UTC().Format("20060102T150405Z"),
		timeoutSec: timeoutSec,
	}
}

func run(cfg config) error {
	if err := os.Chdir(cfg.repoDir); err != nil {
		return fmt.Errorf("chdir %s: %w", cfg.repoDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.timeoutSec)*time.Second)
	defer cancel()

	if err := runSecretPatternScan(); err != nil {
		return err
	}
	if err := runWorkflowPolicyScan(); err != nil {
		return err
	}
	if err := runGoOfficialChecks(ctx); err != nil {
		return err
	}
	return nil
}

func runGoOfficialChecks(ctx context.Context) error {
	fmt.Println("OK: verify-lite go_checks start")

	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go command not found")
	}
	if _, err := exec.LookPath("gofmt"); err != nil {
		return errors.New("gofmt command not found")
	}

	unformatted, err := runCommandCapture(ctx, "gofmt", "-l", ".")
	if err != nil {
		return fmt.Errorf("gofmt -l failed: %w", err)
	}
	unformatted = strings.TrimSpace(unformatted)
	if unformatted != "" {
		return fmt.Errorf("gofmt check failed; unformatted files:\n%s", unformatted)
	}

	if err := runCommand(ctx, "go", "vet", "./..."); err != nil {
		return fmt.Errorf("go vet failed: %w", err)
	}
	if err := runCommand(ctx, "go", "test", "./..."); err != nil {
		return fmt.Errorf("go test failed: %w", err)
	}
	fmt.Println("OK: verify-lite go_checks done")
	return nil
}

func runSecretPatternScan() error {
	fmt.Println("OK: verify-lite secret_scan start")
	patterns := []string{
		"discord.com/api/" + "webhooks/",
		"discordapp.com/api/" + "webhooks/",
		"hooks.slack.com/" + "services/",
	}
	skipDirs := map[string]bool{
		".git":         true,
		"out":          true,
		"cache":        true,
		"tmp":          true,
		"target":       true,
		"node_modules": true,
	}
	found := ""
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(path, ".git/") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > 1024*1024 {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.IndexByte(content, 0) >= 0 {
			return nil
		}
		text := string(content)
		for _, pattern := range patterns {
			if strings.Contains(text, pattern) {
				found = fmt.Sprintf("file=%s pattern=%s", path, pattern)
				return errors.New("secret pattern matched")
			}
		}
		return nil
	})
	if err != nil {
		if found != "" {
			return fmt.Errorf("secret scan matched %s", found)
		}
		return fmt.Errorf("secret scan failed: %w", err)
	}
	fmt.Println("OK: verify-lite secret_scan done")
	return nil
}

func runWorkflowPolicyScan() error {
	fmt.Println("OK: verify-lite workflow_policy_scan start")
	root := filepath.Join(".github", "workflows")
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("SKIP: verify-lite workflow_policy_scan reason=missing_.github/workflows")
			return nil
		}
		return fmt.Errorf("workflow policy scan stat failed: %w", err)
	}
	if !info.IsDir() {
		return errors.New(".github/workflows is not a directory")
	}

	usesRefPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)
	var violations []string

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yml") && !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: read_failed", path))
			return nil
		}
		text := string(content)
		if strings.Contains(text, "pull_request_target:") {
			violations = append(violations, fmt.Sprintf("%s: forbidden pull_request_target", path))
		}

		lines := strings.Split(text, "\n")
		for idx, line := range lines {
			trim := strings.TrimSpace(line)
			if !strings.HasPrefix(trim, "uses:") {
				continue
			}
			ref := strings.TrimSpace(strings.TrimPrefix(trim, "uses:"))
			ref = strings.Trim(ref, `"'`)
			if ref == "" {
				violations = append(violations, fmt.Sprintf("%s:%d empty uses ref", path, idx+1))
				continue
			}
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue
			}
			parts := strings.Split(ref, "@")
			if len(parts) != 2 {
				violations = append(violations, fmt.Sprintf("%s:%d unpinned uses (%s)", path, idx+1, ref))
				continue
			}
			if !usesRefPattern.MatchString(parts[1]) {
				violations = append(violations, fmt.Sprintf("%s:%d non-SHA uses (%s)", path, idx+1, ref))
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("workflow policy scan failed: %w", walkErr)
	}
	if len(violations) > 0 {
		return fmt.Errorf("workflow policy violations: %s", strings.Join(violations, "; "))
	}
	fmt.Println("OK: verify-lite workflow_policy_scan done")
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timeout exceeded (%s %s)", name, strings.Join(args, " "))
	}
	return err
}

func runCommandCapture(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out.String(), fmt.Errorf("timeout exceeded (%s %s)", name, strings.Join(args, " "))
	}
	return out.String(), err
}

func envOr(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envOrInt(key string, fallback int) (int, error) {
	raw := envOr(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s=%q", key, raw)
	}
	return value, nil
}

func writeStatus(cfg config, status, reason string) error {
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return err
	}
	head := "OK"
	switch strings.ToUpper(status) {
	case "ERROR":
		head = "ERROR"
	case "SKIP":
		head = "SKIP"
	}
	lines := []string{
		fmt.Sprintf("%s: verify-lite status=%s", head, status),
		fmt.Sprintf("timestamp=%s", cfg.stamp),
		fmt.Sprintf("status=%s", status),
		fmt.Sprintf("repo_dir=%s", cfg.repoDir),
	}
	if reason != "" {
		lines = append(lines, "ERROR: reason="+reason)
		lines = append(lines, "reason="+reason)
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(cfg.outDir, "verify-lite.status"), []byte(content), 0o644)
}
