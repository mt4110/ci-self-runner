package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: ci_orch panic=%v\n", r)
		}
	}()

	runID := utcStamp()
	runRoot := filepath.Join(".local", "out", "run", runID)
	_ = os.MkdirAll(runRoot, 0o755)

	state := loadState(statePath)
	state.Stop = false
	state.Reason = ""
	state.RunID = runID

	cmd, err := parseCLI(os.Args[1:])
	if err != nil {
		printLine(statusError, "cli", "reason="+err.Error())
		updateState(&state, "cli", statusError, err.Error(), 0, "", "")
		saveState(statePath, state)
		return
	}

	execute(cmd, runRoot, &state)
	saveState(statePath, state)
	if state.Stop {
		printLine(statusError, "plan", "stopped=true")
	} else {
		printLine(statusOK, "plan", "completed=true")
	}
}

func execute(cmd cliCommand, runRoot string, state *stateFile) {
	switch cmd.kind {
	case "help":
		printHelp()
		printLine(statusOK, "help", "usage_shown=true")
	case "one-step":
		executeStep(cmd.targetStep, cmd.timeboxMin, runRoot, state)
	case "run-plan":
		for _, s := range planSteps {
			if state.Stop {
				reason := "reason=STOP already set"
				printLine(statusSkip, string(s), reason)
				updateState(state, string(s), statusSkip, reason, 0, "", "")
				continue
			}
			executeStep(s, cmd.timeboxMin, runRoot, state)
		}
	default:
		printLine(statusError, "cli", "reason=invalid_command_kind")
		state.Stop = true
		state.Reason = "invalid command kind"
	}
}

func executeStep(s step, timeboxMin uint64, runRoot string, state *stateFile) {
	result := runStep(s, timeboxMin, runRoot)
	printStepResult(s, result)
	updateState(state, string(s), result.status, result.reason, result.durationMS, result.logPath, result.command)
	if result.status == statusError {
		state.Stop = true
		state.Reason = fmt.Sprintf("step=%s %s", s, result.reason)
	}
}
