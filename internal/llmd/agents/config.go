package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/assets"
)

// Config is the internal representation of an agent configuration.
type Config struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Role      string   `json:"role,omitempty"`
	MaxBudget float64  `json:"max_budget,omitempty"`
}

// Register writes an agent's config, prompts, and settings to disk.
// Existing files are overwritten for config; prompts and settings
// are only written if they don't already exist (seed behaviour).
func (a *Agents) Register(_ context.Context, cfg Config, _ string) error {
	if cfg.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if cfg.Command == "" {
		return fmt.Errorf("agent command is required")
	}

	dir := filepath.Join(a.dir, cfg.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating agent directory: %w", err)
	}

	// Write config.json (always overwrite).
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding agent config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		return fmt.Errorf("writing agent config: %w", err)
	}

	// Seed default prompt templates (write only if missing).
	for role, content := range assets.Agent.Templates() {
		a.seed(filepath.Join(dir, role+".md"), content)
	}

	// Seed default prompt templates into default/ too.
	defaultDir := filepath.Join(a.dir, "default")
	os.MkdirAll(defaultDir, 0755)
	for role, content := range assets.Agent.Templates() {
		a.seed(filepath.Join(defaultDir, role+".md"), content)
	}

	// Seed runtime settings if available.
	if s := assets.Agent.Settings(cfg.Name); s != "" {
		a.seed(filepath.Join(dir, "settings.json"), s)
	}

	return nil
}

// seed writes content to path only if the file does not exist.
func (a *Agents) seed(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	os.WriteFile(path, []byte(content), 0644)
}

// Settings returns the runtime settings for an agent, or empty
// string if no settings file exists.
func (a *Agents) Settings(_ context.Context, name string) string {
	data, err := os.ReadFile(filepath.Join(a.dir, name, "settings.json"))
	if err != nil {
		return ""
	}
	return string(data)
}

// Remove deletes an agent's directory from disk.
func (a *Agents) Remove(_ context.Context, name, _ string) error {
	dir := filepath.Join(a.dir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return os.RemoveAll(dir)
}

// Agent reads a single agent configuration by name.
func (a *Agents) Agent(_ context.Context, name string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(a.dir, name, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing agent config for %s: %w", name, err)
	}
	return &cfg, nil
}

// Agents returns all registered agent configurations by scanning
// the agents directory for subdirectories containing config.json.
func (a *Agents) Agents(_ context.Context) ([]Config, error) {
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var configs []Config
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "default" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.dir, e.Name(), "config.json"))
		if err != nil {
			continue
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

// Prompt reads a prompt template from disk. Follows the fallback
// chain: .llmd/agents/<name>/<role>.md then
// .llmd/agents/default/<role>.md.
func (a *Agents) Prompt(_ context.Context, name, role string) (string, string, error) {
	// Agent-specific template.
	path := filepath.Join(a.dir, name, role+".md")
	if data, err := os.ReadFile(path); err == nil {
		return string(data), path, nil
	}

	// Default fallback.
	fallback := filepath.Join(a.dir, "default", role+".md")
	if data, err := os.ReadFile(fallback); err == nil {
		return string(data), fallback, nil
	}

	return "", "", fmt.Errorf("%w: no prompt template for %s/%s", ErrNotFound, name, role)
}

// WritePrompt writes a prompt template to disk.
func (a *Agents) WritePrompt(_ context.Context, name, role, content, _ string) error {
	dir := filepath.Join(a.dir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating agent directory: %w", err)
	}
	path := filepath.Join(dir, role+".md")
	return os.WriteFile(path, []byte(content), 0644)
}

// Dir returns the base directory. Used by the host bridge to
// resolve relative paths for display.
func (a *Agents) Dir() string {
	return a.dir
}

// Registered reports whether an agent has a config file on disk.
func (a *Agents) Registered(name string) bool {
	_, err := os.Stat(filepath.Join(a.dir, name, "config.json"))
	return err == nil
}

// Names returns the names of all registered agents (directories
// with a config.json). Excludes "default".
func (a *Agents) Names() []string {
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "default" {
			continue
		}
		if _, err := os.Stat(filepath.Join(a.dir, e.Name(), "config.json")); err == nil {
			names = append(names, e.Name())
		}
	}
	return names
}

// agentDir returns the directory for a named agent, joining paths
// with filepath separators.
func (a *Agents) agentDir(name string) string {
	// Prevent path traversal.
	clean := filepath.Base(name)
	if clean != name || strings.ContainsAny(name, `/\`) {
		return filepath.Join(a.dir, "invalid")
	}
	return filepath.Join(a.dir, name)
}
