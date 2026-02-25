package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	runnerVersion = "2.321.0"
	runnerBaseURL = "https://github.com/actions/runner/releases/download/v" + runnerVersion
	statusOKMsg   = "STATUS: OK"
	statusERRMsg  = "STATUS: ERROR"
)

// SHA256 hashes for runner tarballs (SOT: docs/ci/RUNNER_LOCK.md)
// These should be updated when runner version is bumped.
var runnerHashes = map[string]string{
	"osx-x64":   "", // fill from GitHub releases page
	"osx-arm64": "", // fill from GitHub releases page
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			writeStatus("ERROR", fmt.Sprintf("panic=%v", r))
			fmt.Printf("ERROR: runner_setup panic=%v\n", r)
			fmt.Println(statusERRMsg)
		}
	}()

	apply := hasFlag("--apply")
	if !apply {
		fmt.Println("OK: runner_setup dry-run (pass --apply to execute)")
	}

	arch := detectArch()
	if arch == "" {
		printErrorAndExit("unsupported_architecture=" + runtime.GOARCH)
		return
	}
	fmt.Printf("OK: runner_setup arch=%s version=v%s\n", arch, runnerVersion)

	installDir := filepath.Join(os.Getenv("HOME"), ".local", "ci-runner")

	// Check if already installed
	configPath := filepath.Join(installDir, ".runner")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("OK: runner_setup already_installed dir=%s\n", installDir)
		writeStatus("OK", "already_installed")
		fmt.Println(statusOKMsg)
		return
	}

	if !apply {
		tarball := fmt.Sprintf("actions-runner-%s-%s.tar.gz", arch, runnerVersion)
		downloadURL := runnerBaseURL + "/" + tarball
		fmt.Printf("OK: runner_setup would_download url=%s\n", downloadURL)
		fmt.Printf("OK: runner_setup would_install dir=%s\n", installDir)
		writeStatus("OK", "dry_run")
		fmt.Println(statusOKMsg)
		return
	}

	if err := executeSetup(arch, installDir); err != nil {
		printErrorAndExit(err.Error())
		return
	}

	fmt.Println(statusOKMsg)
}

func executeSetup(arch, installDir string) error {
	tarball := fmt.Sprintf("actions-runner-%s-%s.tar.gz", arch, runnerVersion)
	downloadURL := runnerBaseURL + "/" + tarball
	tarballPath := filepath.Join(installDir, tarball)

	// Create install directory
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("mkdir_failed=%s", err.Error())
	}

	// Download
	fmt.Printf("OK: runner_setup downloading url=%s\n", downloadURL)
	if err := downloadFile(downloadURL, tarballPath); err != nil {
		return fmt.Errorf("download_failed=%s", err.Error())
	}
	fmt.Printf("OK: runner_setup downloaded path=%s\n", tarballPath)

	// SHA256 verify (if hash is available)
	if err := verifySHA256(arch, tarballPath); err != nil {
		return err
	}

	// Extract
	fmt.Printf("OK: runner_setup extracting to=%s\n", installDir)
	if err := runCmd(installDir, "tar", "xzf", tarballPath, "-C", installDir); err != nil {
		return fmt.Errorf("extract_failed=%s", err.Error())
	}
	fmt.Println("OK: runner_setup extracted")

	// Config + service
	return configureAndStartService(installDir)
}

func verifySHA256(arch, tarballPath string) error {
	expectedHash := runnerHashes[arch]
	if expectedHash == "" {
		fmt.Println("SKIP: runner_setup sha256_verify reason=hash_not_configured")
		return nil
	}
	actualHash, err := fileSHA256(tarballPath)
	if err != nil {
		return fmt.Errorf("sha256_read_failed=%s", err.Error())
	}
	if actualHash != expectedHash {
		return fmt.Errorf("sha256_mismatch expected=%s actual=%s", expectedHash, actualHash)
	}
	fmt.Printf("OK: runner_setup sha256_verified=%s\n", actualHash)
	return nil
}

func configureAndStartService(installDir string) error {
	runnerURL := os.Getenv("RUNNER_URL")
	runnerToken := os.Getenv("RUNNER_TOKEN")
	if runnerURL == "" || runnerToken == "" {
		fmt.Println("SKIP: runner_setup config reason=RUNNER_URL_or_RUNNER_TOKEN_not_set")
		fmt.Println("OK: runner_setup extracted_only (set RUNNER_URL and RUNNER_TOKEN to configure)")
		writeStatus("OK", "extracted_only")
		return nil
	}

	// config.sh (token is NOT logged)
	fmt.Println("OK: runner_setup configuring")
	if err := runCmd(installDir, filepath.Join(installDir, "config.sh"),
		"--url", runnerURL,
		"--token", runnerToken,
		"--labels", "self-hosted,mac-mini,colima,verify-full",
		"--runnergroup", "Default",
		"--unattended",
	); err != nil {
		return fmt.Errorf("config_failed=%s", err.Error())
	}
	fmt.Println("OK: runner_setup configured")

	// svc.sh install + start
	fmt.Println("OK: runner_setup installing_service")
	if err := runCmd(installDir, filepath.Join(installDir, "svc.sh"), "install"); err != nil {
		return fmt.Errorf("svc_install_failed=%s", err.Error())
	}
	if err := runCmd(installDir, filepath.Join(installDir, "svc.sh"), "start"); err != nil {
		return fmt.Errorf("svc_start_failed=%s", err.Error())
	}

	fmt.Println("OK: runner_setup service_started")
	writeStatus("OK", "")
	return nil
}

func printErrorAndExit(reason string) {
	writeStatus("ERROR", reason)
	fmt.Printf("ERROR: runner_setup %s\n", reason)
	fmt.Println(statusERRMsg)
}

func detectArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "osx-x64"
	case "arm64":
		return "osx-arm64"
	default:
		return ""
	}
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec // URL is hardcoded from const
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http_status=%d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeStatus(st, reason string) {
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
		fmt.Sprintf("%s: runner-setup status=%s", head, st),
		fmt.Sprintf("timestamp=%s", stamp),
		fmt.Sprintf("status=%s", st),
		fmt.Sprintf("version=v%s", runnerVersion),
		fmt.Sprintf("arch=%s", detectArch()),
	}
	if reason != "" {
		lines = append(lines, "reason="+reason)
	}
	content := strings.Join(lines, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(outDir, "runner-setup.status"), []byte(content), 0o644)
}
