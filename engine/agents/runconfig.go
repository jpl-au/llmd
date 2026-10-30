package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/sdk"
)

const runConfigFile = ".llmd-run.json"

// LogFile is the name of the log of a run, in the directory that holds its run config.
const LogFile = ".llmd-agent.log"

// RunConfig holds the resolved configuration for an agent run. Written
// to the worktree by Spawn and read by the agent run command. This
// replaces the bash wrapper template with a cross-platform JSON file.
type RunConfig struct {
	TaskKey   string `json:"task_key"`
	Role      string `json:"role"`
	OnSuccess string `json:"on_success,omitempty"`
	OnFailure string `json:"on_failure,omitempty"`

	// Harness names the harness that runs the agent, and Spec is what it is given.
	Harness string      `json:"harness"`
	Spec    sdk.RunSpec `json:"spec"`
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
