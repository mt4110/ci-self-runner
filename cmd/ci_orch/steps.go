package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// statusFilePaths maps steps to their SOT status files.
// Steps not listed here fall back to exit-code judgment.
var statusFilePaths = map[step]string{
	stepVerifyLite: "out/verify-lite.status",
	stepFullTest:   "out/verify-full.status",
}

func runStep(s step, timeboxMin uint64, runRoot string) stepResult {
	started := time.Now()
	logPath := filepath.Join(runRoot, string(s)+".log")
	logFile, effectiveLogPath, openErr := createLogFile(logPath)
	if openErr != nil {
		return stepResult{
			status:     statusError,
			reason:     "reason=log_open_failed(" + openErr.Error() + ")",
			durationMS: uint64(time.Since(started).Milliseconds()),
			logPath:    effectiveLogPath,
		}
	}
	defer logFile.Close()

	var result stepResult
	switch s {
	case stepPreflight:
		result = runPreflight(logFile)
	case stepVerifyLite:
		result = runExternalStatusFirst(logFile, "go", []string{"run", "./cmd/verify-lite"}, timeboxMin, statusFilePaths[stepVerifyLite])
	case stepFullBuild:
		result = runExternal(logFile, "docker", []string{"build", "-t", "ci-self-runner:local", "-f", "ci/image/Dockerfile", "."}, timeboxMin)
	case stepFullTest:
		result = runExternalStatusFirst(logFile, "sh", []string{"ops/ci/run_verify_full.sh"}, timeboxMin, statusFilePaths[stepFullTest])
	case stepBundleMake:
		result = runExternal(logFile, "go", []string{"run", "./cmd/review-pack"}, timeboxMin)
	case stepPrCreate:
		result = stepResult{
			status:  statusSkip,
			reason:  "reason=manual_step",
			command: "manual",
		}
	default:
		result = stepResult{
			status:  statusError,
			reason:  "reason=unknown_step",
			command: "internal",
		}
	}

	result.durationMS = uint64(time.Since(started).Milliseconds())
	result.logPath = effectiveLogPath
	return result
}

func runPreflight(logFile *os.File) stepResult {
	required := []string{
		".codex/00-RULES-READ-FIRST.md",
		"docs/ci/SYSTEM.md",
		"docs/ci/FLOW.md",
		"docs/ci/RUNNER_ISOLATION.md",
		"docs/ci/COLIMA_TUNING.md",
		"docs/ci/SHELL_POLICY.md",
		"docs/ci/RUNBOOK.md",
	}

	missingPaths := []string{}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			missingPaths = append(missingPaths, path)
		}
	}

	missingCommands := []string{}
	if !commandAvailable("docker", "--version") {
		missingCommands = append(missingCommands, "docker")
	}
	if !commandAvailable("go", "version") {
		missingCommands = append(missingCommands, "go")
	}

	if len(missingPaths) == 0 && len(missingCommands) == 0 {
		_, _ = fmt.Fprintln(logFile, "preflight: required docs and commands found")
		return stepResult{
			status:  statusOK,
			reason:  "reason=ready",
			command: "internal preflight",
		}
	}

	reasons := []string{}
	if len(missingPaths) > 0 {
		reasons = append(reasons, "missing_paths("+strings.Join(missingPaths, ",")+")")
	}
	if len(missingCommands) > 0 {
		reasons = append(reasons, "missing_commands("+strings.Join(missingCommands, ",")+")")
	}
	reason := "reason=" + strings.Join(reasons, ";")
	_, _ = fmt.Fprintln(logFile, "preflight:", reason)
	return stepResult{
		status:  statusError,
		reason:  reason,
		command: "internal preflight",
	}
}

func commandAvailable(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// runExternalStatusFirst runs an external command and reads the SOT status file
// to determine the result. Exit code is NOT used for judgment.
func runExternalStatusFirst(logFile *os.File, name string, args []string, timeboxMin uint64, statusPath string) stepResult {
	commandText := name + " " + strings.Join(args, " ")
	_, _ = fmt.Fprintf(logFile, "command=%s args=%s\n", name, strings.Join(args, " "))
	_, _ = fmt.Fprintf(logFile, "status_first=true status_path=%s\n", statusPath)

	cmd := exec.Command(name, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return stepResult{
			status:  statusError,
			reason:  "reason=spawn_failed(" + err.Error() + ")",
			command: commandText,
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timeout := time.Duration(timeboxMin) * time.Minute
	select {
	case <-done:
		// Command finished (exit code is intentionally ignored for status-first steps).
		// Read the SOT status file to determine the result.
	case <-time.After(timeout):
		// Graceful shutdown: send SIGINT, wait up to 10s, then give up (no Kill).
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-done:
			// Process exited after SIGINT
		case <-time.After(10 * time.Second):
			// Process did not exit after SIGINT; log but do NOT kill.
			_, _ = fmt.Fprintln(logFile, "ERROR: timebox_exceeded process_did_not_exit_after_sigint")
		}
		return stepResult{
			status:  statusSkip,
			reason:  "reason=timebox_exceeded",
			command: commandText,
		}
	}

	// Read SOT status file
	st := readStatusFile(statusPath)
	return stepResult{
		status:  st,
		reason:  "reason=status_file(" + statusPath + ")",
		command: commandText,
	}
}

// runExternal runs an external command and uses exit code for judgment.
// Used only for steps that do NOT produce a SOT status file (e.g., docker build).
func runExternal(logFile *os.File, name string, args []string, timeboxMin uint64) stepResult {
	commandText := name + " " + strings.Join(args, " ")
	_, _ = fmt.Fprintf(logFile, "command=%s args=%s\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return stepResult{
			status:  statusError,
			reason:  "reason=spawn_failed(" + err.Error() + ")",
			command: commandText,
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timeout := time.Duration(timeboxMin) * time.Minute
	select {
	case err := <-done:
		if err == nil {
			return stepResult{
				status:  statusOK,
				reason:  "reason=command_ok",
				command: commandText,
			}
		}
		return stepResult{
			status:  statusError,
			reason:  "reason=command_failed",
			command: commandText,
		}
	case <-time.After(timeout):
		// Graceful shutdown: send SIGINT, wait up to 10s (no Kill).
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_, _ = fmt.Fprintln(logFile, "ERROR: timebox_exceeded process_did_not_exit_after_sigint")
		}
		return stepResult{
			status:  statusSkip,
			reason:  "reason=timebox_exceeded",
			command: commandText,
		}
	}
}

// readStatusFile parses a status file and returns the status.
// Format: first line containing "status=OK", "status=ERROR", or "status=SKIP".
// If the file is missing or unreadable, returns statusError.
func readStatusFile(path string) status {
	f, err := os.Open(path)
	if err != nil {
		return statusError
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "status=OK") {
			return statusOK
		}
		if strings.Contains(line, "status=ERROR") {
			return statusError
		}
		if strings.Contains(line, "status=SKIP") {
			return statusSkip
		}
	}
	// No status line found — treat as error
	return statusError
}

func createLogFile(path string) (*os.File, string, error) {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		fallbackPath := filepath.Join(".local", "out", "run", "fallback.log")
		_ = os.MkdirAll(filepath.Dir(fallbackPath), 0o755)
		fallbackFile, fallbackErr := os.Create(fallbackPath)
		if fallbackErr != nil {
			return nil, fallbackPath, fmt.Errorf("primary=%v fallback=%v", err, fallbackErr)
		}
		return fallbackFile, fallbackPath, nil
	}
	file, err := os.Create(path)
	if err != nil {
		fallbackPath := filepath.Join(".local", "out", "run", "fallback.log")
		_ = os.MkdirAll(filepath.Dir(fallbackPath), 0o755)
		fallbackFile, fallbackErr := os.Create(fallbackPath)
		if fallbackErr != nil {
			return nil, fallbackPath, fmt.Errorf("primary=%v fallback=%v", err, fallbackErr)
		}
		return fallbackFile, fallbackPath, nil
	}
	return file, path, nil
}

func printLine(st status, stepName, detail string) {
	fmt.Printf("%s: %s %s\n", st, stepName, detail)
}

func printStepResult(stepName step, result stepResult) {
	extra := fmt.Sprintf("%s duration_ms=%d log=%s", result.reason, result.durationMS, result.logPath)
	printLine(result.status, string(stepName), extra)
}
