package main

const statePath = ".local/ci/state.json"

type status string

const (
	statusOK    status = "OK"
	statusSkip  status = "SKIP"
	statusError status = "ERROR"
)

type step string

const (
	stepPreflight  step = "preflight"
	stepVerifyLite step = "verify-lite"
	stepFullBuild  step = "full-build"
	stepFullTest   step = "full-test"
	stepBundleMake step = "bundle-make"
	stepPrCreate   step = "pr-create"
)

var planSteps = []step{
	stepPreflight,
	stepVerifyLite,
	stepFullBuild,
	stepFullTest,
	stepBundleMake,
	stepPrCreate,
}

type stateFile struct {
	Stop       bool        `json:"stop"`
	Reason     string      `json:"reason"`
	UpdatedAt  string      `json:"updated_at"`
	LastStep   string      `json:"last_step"`
	LastStatus string      `json:"last_status"`
	RunID      string      `json:"run_id"`
	Steps      []stateStep `json:"steps"`
}

type stateStep struct {
	RunID      string `json:"run_id"`
	Step       string `json:"step"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Timestamp  string `json:"timestamp"`
	DurationMS uint64 `json:"duration_ms"`
	LogPath    string `json:"log_path"`
	Command    string `json:"command"`
}

type cliCommand struct {
	kind       string
	targetStep step
	timeboxMin uint64
}

type stepResult struct {
	status     status
	reason     string
	durationMS uint64
	logPath    string
	command    string
}
