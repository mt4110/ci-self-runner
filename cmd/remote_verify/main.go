package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	mode            string
	repo            string
	workflow        string
	runLimit        int
	remoteHost      string
	remoteRepo      string
	remoteOutSubdir string
	verifyDryRun    bool
	verifyGHASync   bool
}

type metadata struct {
	sha   string
	ref   string
	runID string
}

type ghRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSHA    string `json:"headSha"`
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: remote_verify panic=%v\n", r)
			fmt.Printf("ERROR: remote_verify stopped\n")
		}
	}()
	run()
}

func run() {
	cfg, ok := parseConfig()
	if !ok {
		return
	}
	fmt.Printf("OK: remote_verify start mode=%s repo=%s\n", cfg.mode, cfg.repo)

	if err := os.Chdir(cfg.repo); err != nil {
		fmt.Printf("ERROR: step=chdir reason=failed path=%s err=%v\n", cfg.repo, err)
		fmt.Printf("ERROR: remote_verify stopped\n")
		return
	}

	md, ok := collectMetadata(cfg.workflow, cfg.runLimit)
	if !ok {
		fmt.Printf("ERROR: remote_verify stopped\n")
		return
	}

	stop := false
	switch cfg.mode {
	case "local":
		if !runLocalVerify(cfg, md) {
			stop = true
		}
	case "remote":
		if !runRemoteVerify(cfg, md) {
			stop = true
		}
		if stop {
			fmt.Printf("SKIP: step=fetch reason=STOP\n")
		} else if !fetchRemoteArtifacts(cfg) {
			stop = true
		}
	default:
		fmt.Printf("ERROR: step=config reason=invalid_mode value=%s\n", cfg.mode)
		stop = true
	}

	if stop {
		fmt.Printf("ERROR: remote_verify stopped\n")
	} else {
		fmt.Printf("OK: remote_verify completed\n")
	}
}

func parseConfig() (config, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("ERROR: step=config reason=getwd_failed err=%v\n", err)
		return config{}, false
	}
	cfg := config{}
	flag.StringVar(&cfg.mode, "mode", "remote", "mode: local or remote")
	flag.StringVar(&cfg.repo, "repo", cwd, "local repository path")
	flag.StringVar(&cfg.workflow, "workflow", "verify.yml", "GitHub Actions workflow file name")
	flag.IntVar(&cfg.runLimit, "run-limit", 30, "gh run list search limit")
	flag.StringVar(&cfg.remoteHost, "remote-host", "macmini", "ssh host alias")
	flag.StringVar(&cfg.remoteRepo, "remote-repo", "/Users/takemuramasaki/dev/ci-self-runner", "remote repository path")
	flag.StringVar(&cfg.remoteOutSubdir, "remote-out-subdir", "out/remote", "local output subdir for fetched artifacts")
	flag.BoolVar(&cfg.verifyDryRun, "verify-dry-run", true, "set VERIFY_DRY_RUN=1")
	flag.BoolVar(&cfg.verifyGHASync, "verify-gha-sync", true, "set VERIFY_GHA_SYNC=1")
	flag.Parse()

	cfg.mode = strings.ToLower(strings.TrimSpace(cfg.mode))
	if cfg.mode != "local" && cfg.mode != "remote" {
		fmt.Printf("ERROR: step=config reason=mode must be local or remote\n")
		return config{}, false
	}
	if cfg.runLimit <= 0 {
		cfg.runLimit = 30
	}
	return cfg, true
}

func collectMetadata(workflow string, limit int) (metadata, bool) {
	md := metadata{}
	sha, err := runCapture("git", "rev-parse", "HEAD")
	if err != nil {
		fmt.Printf("ERROR: step=metadata reason=git_sha_failed err=%v\n", err)
		return metadata{}, false
	}
	ref, err := runCapture("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Printf("ERROR: step=metadata reason=git_ref_failed err=%v\n", err)
		return metadata{}, false
	}
	md.sha = strings.TrimSpace(sha)
	md.ref = strings.TrimSpace(ref)
	fmt.Printf("OK: metadata sha=%s\n", md.sha)
	fmt.Printf("OK: metadata ref=%s\n", md.ref)

	runID, found := resolveRunID(workflow, limit, md.sha)
	if found {
		md.runID = runID
		fmt.Printf("OK: metadata run_id=%s\n", md.runID)
	} else {
		fmt.Printf("SKIP: metadata run_id not found\n")
	}
	return md, true
}

func resolveRunID(workflow string, limit int, sha string) (string, bool) {
	out, err := runCapture("gh", "run", "list", "--workflow", workflow, "--limit", strconv.Itoa(limit), "--json", "databaseId,headSha")
	if err != nil {
		return "", false
	}
	var runs []ghRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return "", false
	}
	for _, r := range runs {
		if strings.EqualFold(r.HeadSHA, sha) {
			return strconv.FormatInt(r.DatabaseID, 10), true
		}
	}
	if len(runs) > 0 {
		return strconv.FormatInt(runs[0].DatabaseID, 10), true
	}
	return "", false
}

func runLocalVerify(cfg config, md metadata) bool {
	fmt.Printf("OK: step=local_verify start\n")
	cmd := exec.Command("sh", "ops/ci/run_verify_full.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"GITHUB_SHA="+md.sha,
		"GITHUB_REF_NAME="+md.ref,
		"GITHUB_RUN_ID="+md.runID,
		"VERIFY_DRY_RUN="+boolAs01(cfg.verifyDryRun),
		"VERIFY_GHA_SYNC="+boolAs01(cfg.verifyGHASync),
		"GITHUB_ACTIONS=true",
	)
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: step=local_verify reason=command_failed err=%v\n", err)
		return false
	}
	fmt.Printf("OK: step=local_verify completed\n")
	return true
}

func runRemoteVerify(cfg config, md metadata) bool {
	fmt.Printf("OK: step=remote_verify start host=%s\n", cfg.remoteHost)
	remoteCmd := fmt.Sprintf(
		"cd %s 2>/dev/null || true; GITHUB_SHA=%s GITHUB_REF_NAME=%s GITHUB_RUN_ID=%s VERIFY_GHA_SYNC=%s VERIFY_DRY_RUN=%s ops/ci/run_verify_full.sh",
		shellQuote(cfg.remoteRepo),
		shellQuote(md.sha),
		shellQuote(md.ref),
		shellQuote(md.runID),
		shellQuote(boolAs01(cfg.verifyGHASync)),
		shellQuote(boolAs01(cfg.verifyDryRun)),
	)
	cmd := exec.Command("ssh", cfg.remoteHost, remoteCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: step=remote_verify reason=ssh_failed err=%v\n", err)
		return false
	}
	fmt.Printf("OK: step=remote_verify completed\n")
	return true
}

func fetchRemoteArtifacts(cfg config) bool {
	localOut := filepath.Join(cfg.repo, cfg.remoteOutSubdir)
	if err := os.MkdirAll(localOut, 0o755); err != nil {
		fmt.Printf("ERROR: step=fetch reason=mkdir_failed path=%s err=%v\n", localOut, err)
		return false
	}

	ok := true
	if err := runStreaming("rsync", "-a", fmt.Sprintf("%s:%s/out/verify-full.status", cfg.remoteHost, cfg.remoteRepo), localOut+"/"); err != nil {
		fmt.Printf("ERROR: step=fetch reason=status_failed err=%v\n", err)
		ok = false
	} else {
		fmt.Printf("OK: step=fetch status=verify-full.status\n")
	}

	logOut := filepath.Join(localOut, "logs")
	if err := os.MkdirAll(logOut, 0o755); err != nil {
		fmt.Printf("ERROR: step=fetch reason=mkdir_logs_failed err=%v\n", err)
		return false
	}
	if err := runStreaming("rsync", "-a", fmt.Sprintf("%s:%s/out/logs/", cfg.remoteHost, cfg.remoteRepo), logOut+"/"); err != nil {
		fmt.Printf("ERROR: step=fetch reason=logs_failed err=%v\n", err)
		ok = false
	} else {
		fmt.Printf("OK: step=fetch logs=%s\n", logOut)
	}
	return ok
}

func runCapture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v failed: %s", name, args, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func boolAs01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
