package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type checkResult struct {
	name   string
	status string // OK, ERROR, SKIP
	detail string
}

type options struct {
	repoDir string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			writeStatus("ERROR", fmt.Sprintf("panic=%v", r), nil)
			fmt.Printf("ERROR: runner_health panic=%v\n", r)
			fmt.Println("STATUS: ERROR")
		}
	}()

	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		writeStatus("ERROR", "invalid_args="+err.Error(), nil)
		fmt.Printf("ERROR: runner_health invalid_args=%s\n", err.Error())
		fmt.Println("STATUS: ERROR")
		os.Exit(2)
	}

	fmt.Println("OK: runner_health start")

	results := []checkResult{}
	overallStatus := "OK"

	// 1) Check gh CLI
	results = append(results, checkCommand("gh", "gh", "version"))

	// 2) Check go
	results = append(results, checkCommand("go", "go", "version"))

	// 3) Check colima
	results = append(results, checkColima())

	// 4) Check docker
	results = append(results, checkCommand("docker", "docker", "info"))

	// 5) Check disk usage for key directories
	results = append(results, checkDiskDir("out"))
	results = append(results, checkDiskDir(".local"))
	results = append(results, checkDiskDir("cache"))
	results = append(results, checkNixForRepo(opts.repoDir))

	// Print results and determine overall status
	for _, r := range results {
		fmt.Printf("%s: runner_health check=%s %s\n", r.status, r.name, r.detail)
		if r.status == "ERROR" {
			overallStatus = "ERROR"
		}
	}

	writeStatus(overallStatus, "", results)
	fmt.Printf("STATUS: %s\n", overallStatus)
}

func parseOptions(args []string) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("runner_health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.repoDir, "repo-dir", "", "target repository directory for context-aware checks")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected_args=%s", strings.Join(fs.Args(), ","))
	}
	return opts, nil
}

func checkCommand(name, bin string, args ...string) checkResult {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return checkResult{
			name:   name,
			status: "ERROR",
			detail: "reason=not_available(" + err.Error() + ")",
		}
	}
	return checkResult{
		name:   name,
		status: "OK",
		detail: "reason=available",
	}
}

func checkColima() checkResult {
	// Check if colima is installed
	if _, err := exec.LookPath("colima"); err != nil {
		return checkResult{
			name:   "colima",
			status: "ERROR",
			detail: "reason=not_installed",
		}
	}

	// Check colima status
	cmd := exec.Command("colima", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return checkResult{
			name:   "colima",
			status: "ERROR",
			detail: "reason=not_running(" + strings.TrimSpace(string(out)) + ")",
		}
	}
	return checkResult{
		name:   "colima",
		status: "OK",
		detail: "reason=running",
	}
}

func checkDiskDir(dir string) checkResult {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				name:   "disk_" + dir,
				status: "SKIP",
				detail: "reason=dir_not_found",
			}
		}
		return checkResult{
			name:   "disk_" + dir,
			status: "ERROR",
			detail: "reason=stat_failed(" + err.Error() + ")",
		}
	}
	if !info.IsDir() {
		return checkResult{
			name:   "disk_" + dir,
			status: "ERROR",
			detail: "reason=not_a_directory",
		}
	}

	// Count files and approximate total size
	var totalSize int64
	var fileCount int
	_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			totalSize += fi.Size()
			fileCount++
		}
		return nil
	})

	sizeMB := totalSize / (1024 * 1024)
	detail := fmt.Sprintf("files=%d size_mb=%d", fileCount, sizeMB)

	// Warn if directory is large (> 1GB)
	st := "OK"
	if sizeMB > 1024 {
		st = "ERROR"
		detail += " reason=directory_too_large(>1GB)"
	}

	return checkResult{
		name:   "disk_" + dir,
		status: st,
		detail: detail,
	}
}

func checkNixForRepo(repoDir string) checkResult {
	if strings.TrimSpace(repoDir) == "" {
		return checkResult{
			name:   "nix",
			status: "SKIP",
			detail: "reason=repo_dir_not_set",
		}
	}

	flakePath := filepath.Join(repoDir, "flake.nix")
	fi, err := os.Stat(flakePath)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				name:   "nix",
				status: "SKIP",
				detail: "reason=not_required(no_flake)",
			}
		}
		return checkResult{
			name:   "nix",
			status: "ERROR",
			detail: "reason=flake_stat_failed(" + err.Error() + ")",
		}
	}
	if fi.IsDir() {
		return checkResult{
			name:   "nix",
			status: "ERROR",
			detail: "reason=flake_not_file",
		}
	}

	// flake.nix exists => nix is required.
	if p, err := exec.LookPath("nix"); err == nil {
		return checkResult{
			name:   "nix",
			status: "OK",
			detail: "reason=available path=" + p,
		}
	}

	defaultProfileBin := "/nix/var/nix/profiles/default/bin/nix"
	if _, err := os.Stat(defaultProfileBin); err == nil {
		return checkResult{
			name:   "nix",
			status: "OK",
			detail: "reason=available_via_default_profile path=" + defaultProfileBin,
		}
	}

	user := strings.TrimSpace(os.Getenv("USER"))
	if user == "" {
		user = "unknown"
	}
	perUserBin := filepath.Join("/nix/var/nix/profiles/per-user", user, "profile/bin/nix")
	if _, err := os.Stat(perUserBin); err == nil {
		return checkResult{
			name:   "nix",
			status: "OK",
			detail: "reason=available_via_user_profile path=" + perUserBin,
		}
	}

	daemonProfile := "/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh"
	if _, err := os.Stat(daemonProfile); err == nil {
		return checkResult{
			name:   "nix",
			status: "ERROR",
			detail: "reason=not_in_path profile=" + daemonProfile,
		}
	}

	return checkResult{
		name:   "nix",
		status: "ERROR",
		detail: "reason=not_installed",
	}
}

func writeStatus(st string, reason string, results []checkResult) {
	outDir := "out"
	_ = os.MkdirAll(outDir, 0o755)
	stamp := time.Now().UTC().Format("20060102T150405Z")
	head := "OK"
	switch strings.ToUpper(st) {
	case "ERROR":
		head = "ERROR"
	case "SKIP":
		head = "SKIP"
	}
	lines := []string{
		fmt.Sprintf("%s: runner_health status=%s", head, st),
		fmt.Sprintf("timestamp=%s", stamp),
		fmt.Sprintf("status=%s", st),
	}
	if reason != "" {
		lines = append(lines, "reason="+reason)
	}
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("%s: check=%s %s", r.status, r.name, r.detail))
	}
	content := strings.Join(lines, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(outDir, "health.status"), []byte(content), 0o644)
}
