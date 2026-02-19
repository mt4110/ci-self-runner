package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
		result = runExternal(logFile, "go", []string{"run", "./cmd/verify-lite"}, timeboxMin)
	case stepFullBuild:
		result = runExternal(logFile, "docker", []string{"build", "-t", "ci-self-runner:local", "-f", "ci/image/Dockerfile", "."}, timeboxMin)
	case stepFullTest:
		result = runExternal(logFile, "sh", []string{"ops/ci/run_verify_full.sh"}, timeboxMin)
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
		_ = cmd.Process.Kill()
		<-done
		return stepResult{
			status:  statusSkip,
			reason:  "reason=timebox_exceeded",
			command: commandText,
		}
	}
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
