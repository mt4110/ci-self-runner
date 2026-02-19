package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRunWithoutReadme(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	outDir := filepath.Join(tmp, "out")
	cacheDir := filepath.Join(tmp, "cache")
	mustMkdirAll(t, repoDir, outDir, cacheDir)

	cfg := config{
		repoDir:  repoDir,
		outDir:   outDir,
		cacheDir: cacheDir,
		stamp:    "20260219T000000Z",
	}
	opts := options{
		dryRun: true,
	}

	var buf bytes.Buffer
	if err := run(cfg, opts, &buf); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	status := mustRead(t, filepath.Join(outDir, "verify-full.status"))
	if !strings.Contains(status, "status=OK") {
		t.Fatalf("status file does not contain OK: %s", status)
	}
	if !strings.Contains(status, "mode=dry-run") {
		t.Fatalf("status file does not contain dry-run mode: %s", status)
	}
	if !strings.Contains(buf.String(), "dry-run enabled") {
		t.Fatalf("stdout does not contain dry-run message: %s", buf.String())
	}
}

func TestRunFullRequiresReadme(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	outDir := filepath.Join(tmp, "out")
	cacheDir := filepath.Join(tmp, "cache")
	mustMkdirAll(t, repoDir, outDir, cacheDir)

	cfg := config{
		repoDir:  repoDir,
		outDir:   outDir,
		cacheDir: cacheDir,
		stamp:    "20260219T000001Z",
	}
	opts := options{}

	err := run(cfg, opts, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run should fail without README.md in full mode")
	}
	if !strings.Contains(err.Error(), "README.md not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGhaSyncWritesMetadata(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	outDir := filepath.Join(tmp, "out")
	cacheDir := filepath.Join(tmp, "cache")
	mustMkdirAll(t, repoDir, outDir, cacheDir)
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	cfg := config{
		repoDir:  repoDir,
		outDir:   outDir,
		cacheDir: cacheDir,
		stamp:    "20260219T000002Z",
	}
	opts := options{
		ghaSync:     true,
		githubRunID: "12345",
		githubSHA:   "abcdef",
		githubRef:   "main",
	}

	var buf bytes.Buffer
	if err := run(cfg, opts, &buf); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	stdout := buf.String()
	if !strings.Contains(stdout, "::notice::verify-full start") {
		t.Fatalf("stdout does not contain start annotation: %s", stdout)
	}
	if !strings.Contains(stdout, "::notice::verify-full done") {
		t.Fatalf("stdout does not contain done annotation: %s", stdout)
	}

	status := mustRead(t, filepath.Join(outDir, "verify-full.status"))
	if !strings.Contains(status, "gha_sync=true") {
		t.Fatalf("status file does not contain gha_sync=true: %s", status)
	}
	if !strings.Contains(status, "github_run_id=12345") {
		t.Fatalf("status file does not contain github_run_id: %s", status)
	}
	if !strings.Contains(status, "github_sha=abcdef") {
		t.Fatalf("status file does not contain github_sha: %s", status)
	}
	if !strings.Contains(status, "github_ref=main") {
		t.Fatalf("status file does not contain github_ref: %s", status)
	}
}

func mustMkdirAll(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
