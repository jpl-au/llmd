package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const runConfigFile = ".llmd-run.json"

// RunConfig holds the resolved configuration for an agent run. Written
// to the worktree by Spawn and read by the agent run command. This
// replaces the bash wrapper template with a cross-platform JSON file.
type RunConfig struct {
	TaskKey   string   `json:"task_key"`
	Agent     string   `json:"agent"`
	Role      string   `json:"role"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	OnSuccess string   `json:"on_success,omitempty"`
	OnFailure string   `json:"on_failure,omitempty"`
}

// WriteRunConfig serialises the config to .llmd-run.json in the given
// worktree directory.
func WriteRunConfig(worktree string, cfg RunConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling run config: %w", err)
	}
	path := filepath.Join(worktree, runConfigFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing run config: %w", err)
	}
	return nil
}

// ReadRunConfig reads .llmd-run.json from the given worktree directory.
func ReadRunConfig(worktree string) (*RunConfig, error) {
	path := filepath.Join(worktree, runConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading run config: %w", err)
	}
	var cfg RunConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing run config: %w", err)
	}
	return &cfg, nil
}
