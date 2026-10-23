package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is the session saved between runs: open tabs, active panel and
// command line history.
type State struct {
	Active      int          `json:"active"`
	Panels      []PanelState `json:"panels"`
	History     []string     `json:"history,omitempty"`
	DirHistory  []string     `json:"dir_history,omitempty"`  // directories visited, oldest first
	FileHistory []string     `json:"file_history,omitempty"` // files viewed / edited / opened, oldest first
	Heights     []int        `json:"heights,omitempty"`      // rows taken off each panel's height
	Split       int          `json:"split,omitempty"`        // divider shift from the middle
}

type PanelState struct {
	Active int      `json:"active"`
	Tabs   []string `json:"tabs"`
}

func statePath(dir string) string { return filepath.Join(dir, "state.json") }

func loadState(dir string) *State {
	data, err := os.ReadFile(statePath(dir))
	if err != nil {
		return nil
	}
	var st State
	if json.Unmarshal(data, &st) != nil {
		return nil
	}
	return &st
}

func saveState(dir string, st *State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dir, 0o755)
	return os.WriteFile(statePath(dir), data, 0o644)
}
