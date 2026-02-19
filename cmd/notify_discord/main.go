package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type config struct {
	statusPath string
	title      string
	dryRun     bool
	webhookEnv string
	minLevel   string
	webhookURL string
}

type statusSummary struct {
	level string
	lines []string
}

type discordPayload struct {
	Content string `json:"content"`
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: notify_discord panic=%v\n", r)
		}
	}()

	cfg := parseConfig()
	if cfg.webhookURL == "" {
		fmt.Printf("ERROR: notify_discord webhook_missing env=%s\n", cfg.webhookEnv)
		return
	}

	summary, err := parseStatusFile(cfg.statusPath)
	if err != nil {
		fmt.Printf("ERROR: notify_discord parse_status err=%s\n", err.Error())
		return
	}
	if !shouldNotify(summary.level, cfg.minLevel) {
		fmt.Printf("SKIP: notify_discord level=%s min=%s\n", summary.level, cfg.minLevel)
		return
	}
	content := buildContent(cfg.title, summary)

	if cfg.dryRun {
		fmt.Println("OK: notify_discord dry_run payload")
		fmt.Println(content)
		fmt.Println("OK: notify_discord completed")
		return
	}

	if err := sendDiscord(cfg.webhookURL, content); err != nil {
		fmt.Printf("ERROR: notify_discord send err=%s\n", err.Error())
		return
	}
	fmt.Println("OK: notify_discord sent")
	fmt.Println("OK: notify_discord completed")
}

func parseConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.statusPath, "status", "out/verify-full.status", "status file path")
	flag.StringVar(&cfg.title, "title", "verify-full", "notification title")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "print payload only")
	flag.StringVar(&cfg.webhookEnv, "webhook-env", "DISCORD_WEBHOOK_URL", "webhook environment variable name")
	flag.StringVar(&cfg.minLevel, "min-level", "ERROR", "minimum level to notify: OK|SKIP|ERROR")
	flag.Parse()
	cfg.webhookEnv = strings.TrimSpace(cfg.webhookEnv)
	if cfg.webhookEnv == "" {
		cfg.webhookEnv = "DISCORD_WEBHOOK_URL"
	}
	cfg.minLevel = normalizeLevel(cfg.minLevel)
	cfg.webhookURL = strings.TrimSpace(os.Getenv(cfg.webhookEnv))
	return cfg
}

func parseStatusFile(path string) (statusSummary, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return statusSummary{}, err
	}

	lines := strings.Split(string(content), "\n")
	relevant := make([]string, 0, 20)
	level := ""
	fallbackStatus := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ERROR:") {
			if len(relevant) < 20 {
				relevant = append(relevant, line)
			}
			level = "ERROR"
			continue
		}
		if strings.HasPrefix(line, "SKIP:") {
			if len(relevant) < 20 {
				relevant = append(relevant, line)
			}
			if level != "ERROR" {
				level = "SKIP"
			}
			continue
		}
		if strings.HasPrefix(line, "OK:") {
			if len(relevant) < 20 {
				relevant = append(relevant, line)
			}
			if level == "" {
				level = "OK"
			}
			continue
		}
		if strings.HasPrefix(line, "status=") {
			fallbackStatus = strings.ToUpper(strings.TrimPrefix(line, "status="))
		}
	}

	if level == "" {
		switch fallbackStatus {
		case "ERROR":
			level = "ERROR"
		case "SKIP":
			level = "SKIP"
		case "OK":
			level = "OK"
		default:
			level = "SKIP"
			relevant = append(relevant, "SKIP: no OK/SKIP/ERROR lines found in status file")
		}
	}

	return statusSummary{level: level, lines: relevant}, nil
}

func buildContent(title string, summary statusSummary) string {
	rows := []string{
		fmt.Sprintf("%s: %s", summary.level, title),
		fieldLine("repo", os.Getenv("GITHUB_REPOSITORY")),
		fieldLine("ref", os.Getenv("GITHUB_REF_NAME")),
		fieldLine("sha", os.Getenv("GITHUB_SHA")),
		fieldLine("run_id", os.Getenv("GITHUB_RUN_ID")),
	}

	runURL := buildRunURL()
	if runURL == "" {
		rows = append(rows, "SKIP: run_url missing")
	} else {
		rows = append(rows, "OK: run_url="+runURL)
	}

	for _, line := range summary.lines {
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func fieldLine(name, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "SKIP: " + name + " missing"
	}
	return "OK: " + name + "=" + trimmed
}

func buildRunURL() string {
	serverURL := strings.TrimSpace(os.Getenv("GITHUB_SERVER_URL"))
	repo := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	runID := strings.TrimSpace(os.Getenv("GITHUB_RUN_ID"))
	if serverURL == "" || repo == "" || runID == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", serverURL, repo, runID)
}

func sendDiscord(webhookURL, content string) error {
	payloadBytes, err := json.Marshal(discordPayload{Content: content})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func shouldNotify(level, minLevel string) bool {
	return levelRank(normalizeLevel(level)) >= levelRank(normalizeLevel(minLevel))
}

func levelRank(level string) int {
	switch normalizeLevel(level) {
	case "OK":
		return 1
	case "SKIP":
		return 2
	case "ERROR":
		return 3
	default:
		return 0
	}
}

func normalizeLevel(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "OK":
		return "OK"
	case "SKIP":
		return "SKIP"
	default:
		return "ERROR"
	}
}
