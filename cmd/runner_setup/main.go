package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
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
	defaultLabels = "self-hosted,mac-mini,colima,verify-full"
)

// SHA256 hashes for runner tarballs (SOT: docs/ci/RUNNER_LOCK.md)
// These should be updated when runner version is bumped.
var runnerHashes = map[string]string{
	"osx-x64":   "", // fill from GitHub releases page
	"osx-arm64": "", // fill from GitHub releases page
}

type options struct {
	apply      bool
	repo       string
	installDir string
	labels     string
	runnerName string
	group      string
	noService  bool
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			writeStatus("ERROR", fmt.Sprintf("panic=%v", r))
			fmt.Printf("ERROR: runner_setup panic=%v\n", r)
			fmt.Println(statusERRMsg)
		}
	}()

	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		printUsage()
		printErrorAndExit("invalid_args=" + err.Error())
		return
	}

	if !opts.apply {
		fmt.Println("OK: runner_setup dry-run (pass --apply to execute)")
	}

	arch := detectArch()
	if arch == "" {
		printErrorAndExit("unsupported_architecture=" + runtime.GOARCH)
		return
	}
	fmt.Printf("OK: runner_setup arch=%s version=v%s\n", arch, runnerVersion)
	if opts.repo != "" {
		fmt.Printf("OK: runner_setup repo=%s\n", opts.repo)
	}
	fmt.Printf("OK: runner_setup install_dir=%s\n", opts.installDir)

	// Check if already installed
	configPath := filepath.Join(opts.installDir, ".runner")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("OK: runner_setup already_installed dir=%s\n", opts.installDir)
		writeStatus("OK", "already_installed")
		fmt.Println(statusOKMsg)
		return
	}

	if !opts.apply {
		tarball := fmt.Sprintf("actions-runner-%s-%s.tar.gz", arch, runnerVersion)
		downloadURL := runnerBaseURL + "/" + tarball
		fmt.Printf("OK: runner_setup would_download url=%s\n", downloadURL)
		fmt.Printf("OK: runner_setup would_install dir=%s\n", opts.installDir)
		if opts.repo != "" {
			fmt.Printf("OK: runner_setup would_config url=https://github.com/%s labels=%s group=%s\n", opts.repo, opts.labels, opts.group)
			if os.Getenv("RUNNER_TOKEN") == "" {
				fmt.Println("OK: runner_setup would_fetch_registration_token via=gh_api")
			}
		}
		writeStatus("OK", "dry_run")
		fmt.Println(statusOKMsg)
		return
	}

	if err := executeSetup(arch, opts); err != nil {
		printErrorAndExit(err.Error())
		return
	}

	fmt.Println(statusOKMsg)
}

func parseOptions(args []string) (options, error) {
	opts := options{
		labels: defaultLabels,
		group:  "Default",
	}

	fs := flag.NewFlagSet("runner_setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.apply, "apply", false, "execute setup")
	fs.StringVar(&opts.repo, "repo", "", "target repository (owner/repo)")
	fs.StringVar(&opts.installDir, "install-dir", "", "runner install directory")
	fs.StringVar(&opts.labels, "labels", defaultLabels, "comma-separated labels")
	fs.StringVar(&opts.runnerName, "name", "", "runner name")
	fs.StringVar(&opts.group, "runner-group", "Default", "runner group")
	fs.BoolVar(&opts.noService, "no-service", false, "skip svc.sh install/start")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected_args=%s", strings.Join(fs.Args(), ","))
	}

	if opts.repo == "" {
		opts.repo = strings.TrimSpace(os.Getenv("RUNNER_REPO"))
	}
	if opts.installDir == "" {
		opts.installDir = defaultInstallDir(opts.repo)
	}
	return opts, nil
}

func printUsage() {
	fmt.Println("Usage: go run ./cmd/runner_setup [--apply] [--repo owner/repo] [--install-dir <dir>] [--labels <csv>] [--name <runner-name>] [--runner-group <group>] [--no-service]")
}

func defaultInstallDir(repo string) string {
	base := filepath.Join(os.Getenv("HOME"), ".local")
	if repo == "" {
		return filepath.Join(base, "ci-runner")
	}
	return filepath.Join(base, "ci-runner-"+sanitizeForPath(repo))
}

func sanitizeForPath(s string) string {
	if s == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}

func executeSetup(arch string, opts options) error {
	tarball := fmt.Sprintf("actions-runner-%s-%s.tar.gz", arch, runnerVersion)
	downloadURL := runnerBaseURL + "/" + tarball
	tarballPath := filepath.Join(opts.installDir, tarball)

	// Create install directory
	if err := os.MkdirAll(opts.installDir, 0o755); err != nil {
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
	fmt.Printf("OK: runner_setup extracting to=%s\n", opts.installDir)
	if err := runCmd(opts.installDir, "tar", "xzf", tarballPath, "-C", opts.installDir); err != nil {
		return fmt.Errorf("extract_failed=%s", err.Error())
	}
	fmt.Println("OK: runner_setup extracted")

	// Config + service
	return configureAndStartService(opts)
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

func configureAndStartService(opts options) error {
	installDir := opts.installDir
	runnerURL := strings.TrimSpace(os.Getenv("RUNNER_URL"))
	if runnerURL == "" && opts.repo != "" {
		runnerURL = "https://github.com/" + opts.repo
	}

	runnerToken := strings.TrimSpace(os.Getenv("RUNNER_TOKEN"))
	if runnerToken == "" && opts.repo != "" {
		fmt.Printf("OK: runner_setup fetching_registration_token repo=%s\n", opts.repo)
		token, err := fetchRepoRegistrationToken(opts.repo)
		if err != nil {
			return err
		}
		runnerToken = token
		fmt.Println("OK: runner_setup fetched_registration_token")
	}

	if runnerURL == "" || runnerToken == "" {
		fmt.Println("SKIP: runner_setup config reason=RUNNER_URL_or_RUNNER_TOKEN_not_set")
		fmt.Println("OK: runner_setup extracted_only (set RUNNER_URL and RUNNER_TOKEN or use --repo to auto-resolve)")
		writeStatus("OK", "extracted_only")
		return nil
	}

	runnerName := strings.TrimSpace(opts.runnerName)
	if runnerName == "" {
		runnerName = defaultRunnerName(opts.repo)
	}

	// config.sh (token is NOT logged)
	fmt.Println("OK: runner_setup configuring")
	if err := runCmd(installDir, filepath.Join(installDir, "config.sh"),
		"--url", runnerURL,
		"--token", runnerToken,
		"--labels", opts.labels,
		"--runnergroup", opts.group,
		"--name", runnerName,
		"--unattended",
	); err != nil {
		return fmt.Errorf("config_failed=%s", err.Error())
	}
	fmt.Println("OK: runner_setup configured")

	if opts.noService {
		fmt.Println("SKIP: runner_setup service reason=no_service_flag")
		writeStatus("OK", "configured_no_service")
		return nil
	}

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

func fetchRepoRegistrationToken(repo string) (string, error) {
	if !strings.Contains(repo, "/") {
		return "", fmt.Errorf("invalid_repo=%s", repo)
	}
	cmd := exec.Command("gh", "api", "--method", "POST", fmt.Sprintf("repos/%s/actions/runners/registration-token", repo), "--jq", ".token")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("token_fetch_failed=%s", msg)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("token_fetch_empty")
	}
	return token, nil
}

func defaultRunnerName(repo string) string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "mac-mini"
	}
	host = sanitizeForPath(host)
	if repo == "" {
		return host
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return host + "-runner"
	}
	return host + "-" + sanitizeForPath(parts[1])
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
