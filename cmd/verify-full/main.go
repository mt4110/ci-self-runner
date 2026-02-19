package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type config struct {
	repoDir  string
	outDir   string
	cacheDir string
	stamp    string
}

type options struct {
	dryRun      bool
	ghaSync     bool
	githubRunID string
	githubSHA   string
	githubRef   string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			cfg := defaultConfig()
			opts := options{
				ghaSync:     parseBool(os.Getenv("VERIFY_GHA_SYNC")) || strings.EqualFold(os.Getenv("GITHUB_ACTIONS"), "true"),
				githubRunID: os.Getenv("GITHUB_RUN_ID"),
				githubSHA:   os.Getenv("GITHUB_SHA"),
				githubRef:   os.Getenv("GITHUB_REF_NAME"),
			}
			writeErrorStatus(cfg, opts, fmt.Sprintf("panic=%v", r))
			fmt.Printf("ERROR: verify-full panic=%v\n", r)
			fmt.Println("STATUS: ERROR")
		}
	}()

	cfg := defaultConfig()
	opts, err := parseOptions(os.Args[1:], os.Getenv)
	if err != nil {
		writeErrorStatus(cfg, opts, err.Error())
		fmt.Printf("ERROR: verify-full parse_options err=%s\n", err.Error())
		fmt.Println("STATUS: ERROR")
		return
	}

	if err := run(cfg, opts, os.Stdout); err != nil {
		writeErrorStatus(cfg, opts, err.Error())
		if opts.ghaSync {
			fmt.Printf("::error::%s\n", escapeAnnotation(err.Error()))
		}
		fmt.Printf("ERROR: verify-full run err=%s\n", err.Error())
		fmt.Println("STATUS: ERROR")
		return
	}

	fmt.Println("OK: verify-full completed")
	fmt.Println("STATUS: OK")
}

func parseOptions(args []string, getenv func(string) string) (options, error) {
	fs := flag.NewFlagSet("verify-full", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	dryRunDefault := parseBool(getenv("VERIFY_DRY_RUN"))
	ghaSyncDefault := parseBool(getenv("VERIFY_GHA_SYNC")) || strings.EqualFold(getenv("GITHUB_ACTIONS"), "true")

	dryRun := fs.Bool("dry-run", dryRunDefault, "run in dry-run mode")
	ghaSync := fs.Bool("gha-sync", ghaSyncDefault, "emit GitHub Actions compatible annotations")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if len(fs.Args()) > 0 {
		return options{}, errors.New("unexpected positional arguments")
	}

	return options{
		dryRun:      *dryRun,
		ghaSync:     *ghaSync,
		githubRunID: getenv("GITHUB_RUN_ID"),
		githubSHA:   getenv("GITHUB_SHA"),
		githubRef:   getenv("GITHUB_REF_NAME"),
	}, nil
}

func run(cfg config, opts options, stdout io.Writer) error {
	const missingDirFmt = "missing %s"

	if err := requireDir(cfg.repoDir); err != nil {
		return fmt.Errorf(missingDirFmt, cfg.repoDir)
	}
	if err := requireDir(cfg.outDir); err != nil {
		return fmt.Errorf(missingDirFmt, cfg.outDir)
	}
	if err := requireDir(cfg.cacheDir); err != nil {
		return fmt.Errorf(missingDirFmt, cfg.cacheDir)
	}

	logDir := filepath.Join(cfg.outDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}

	logFilePath := filepath.Join(logDir, "verify-full-"+cfg.stamp+".log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	writer := io.MultiWriter(stdout, logFile)
	if opts.ghaSync {
		fmt.Fprintln(writer, "::notice::verify-full start")
	}
	fmt.Fprintf(writer, "OK: verify-full started stamp=%s\n", cfg.stamp)
	fmt.Fprintf(writer, "OK: repo=%s\n", cfg.repoDir)
	fmt.Fprintf(writer, "OK: out=%s\n", cfg.outDir)
	fmt.Fprintf(writer, "OK: cache=%s\n", cfg.cacheDir)
	fmt.Fprintf(writer, "OK: mode=%s\n", modeValue(opts.dryRun))
	fmt.Fprintf(writer, "OK: gha_sync=%t\n", opts.ghaSync)

	if !opts.dryRun {
		if _, err := os.Stat(filepath.Join(cfg.repoDir, "README.md")); err != nil {
			return fmt.Errorf("README.md not found in %s", cfg.repoDir)
		}
	} else {
		fmt.Fprintln(writer, "SKIP: readme_check reason=dry_run (dry-run enabled)")
	}

	if err := writeStatus(cfg, opts, "OK", ""); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	deleted, gcErr := trimLogs(filepath.Join(cfg.outDir, "logs"), 5)
	if gcErr != nil {
		fmt.Fprintf(writer, "ERROR: auto_gc_logs err=%s\n", gcErr.Error())
	} else {
		fmt.Fprintf(writer, "OK: auto_gc_logs deleted=%d keep=5\n", deleted)
	}

	fmt.Fprintln(writer, "OK: verify-full completed")
	if opts.ghaSync {
		fmt.Fprintln(writer, "::notice::verify-full done")
	}
	return nil
}

func defaultConfig() config {
	return config{
		repoDir:  envOr("REPO_DIR", "/repo"),
		outDir:   envOr("OUT_DIR", "/out"),
		cacheDir: envOr("CACHE_DIR", "/cache"),
		stamp:    time.Now().UTC().Format("20060102T150405Z"),
	}
}

func writeErrorStatus(cfg config, opts options, reason string) {
	if _, err := os.Stat(cfg.outDir); err != nil {
		return
	}
	_ = writeStatus(cfg, opts, "ERROR", reason)
}

func requireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not directory", path)
	}
	return nil
}

func envOr(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func modeValue(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}
	return "full"
}

func writeStatus(cfg config, opts options, status, reason string) error {
	head := "OK"
	switch strings.ToUpper(status) {
	case "ERROR":
		head = "ERROR"
	case "SKIP":
		head = "SKIP"
	}
	lines := []string{
		fmt.Sprintf("%s: verify-full status=%s mode=%s", head, status, modeValue(opts.dryRun)),
		fmt.Sprintf("timestamp=%s", cfg.stamp),
		fmt.Sprintf("status=%s", status),
		fmt.Sprintf("mode=%s", modeValue(opts.dryRun)),
		fmt.Sprintf("gha_sync=%t", opts.ghaSync),
	}
	if opts.githubRunID != "" {
		lines = append(lines, "OK: github_run_id="+opts.githubRunID)
		lines = append(lines, "github_run_id="+opts.githubRunID)
	}
	if opts.githubSHA != "" {
		lines = append(lines, "OK: github_sha="+opts.githubSHA)
		lines = append(lines, "github_sha="+opts.githubSHA)
	}
	if opts.githubRef != "" {
		lines = append(lines, "OK: github_ref="+opts.githubRef)
		lines = append(lines, "github_ref="+opts.githubRef)
	}
	if reason != "" {
		lines = append(lines, "ERROR: reason="+reason)
		lines = append(lines, "reason="+reason)
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(cfg.outDir, "verify-full.status"), []byte(content), 0o644)
}

func escapeAnnotation(message string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(message)
}

func trimLogs(logDir string, keep int) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}
	type item struct {
		path    string
		modTime time.Time
	}
	items := []item{}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		items = append(items, item{
			path:    filepath.Join(logDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].modTime.After(items[j].modTime) })
	if len(items) <= keep {
		return 0, nil
	}
	deleted := 0
	for _, old := range items[keep:] {
		if removeErr := os.RemoveAll(old.path); removeErr != nil {
			return deleted, removeErr
		}
		deleted++
	}
	return deleted, nil
}
