package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func loadState(path string) stateFile {
	content, err := os.ReadFile(path)
	if err != nil {
		return stateFile{}
	}
	var st stateFile
	if err := json.Unmarshal(content, &st); err != nil {
		return stateFile{}
	}
	return st
}

func saveState(path string, st stateFile) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	bytes, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, bytes, 0o644)
}

func updateState(st *stateFile, stepName string, stValue status, reason string, durationMS uint64, logPath string, command string) {
	st.UpdatedAt = nowEpochString()
	st.LastStep = stepName
	st.LastStatus = string(stValue)
	if stValue == statusError {
		st.Stop = true
		st.Reason = reason
	}
	st.Steps = append(st.Steps, stateStep{
		RunID:      st.RunID,
		Step:       stepName,
		Status:     string(stValue),
		Reason:     reason,
		Timestamp:  nowEpochString(),
		DurationMS: durationMS,
		LogPath:    logPath,
		Command:    command,
	})
}

func utcStamp() string {
	now := time.Now().UTC()
	return fmt.Sprintf("run-%d-%03d", now.Unix(), now.Nanosecond()/1_000_000)
}

func nowEpochString() string {
	return fmt.Sprintf("%d", time.Now().UTC().Unix())
}
