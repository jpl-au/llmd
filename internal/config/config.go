// Package config handles configuration loading and saving for llmd.
//
// Configuration is loaded from two locations, with local overriding global:
//
//	~/.llmd/config.yaml       # Global (user-wide defaults)
//	.llmd/config.yaml         # Local (project-specific)
//
// Resolution order (highest priority first):
//  1. CLI flags (--author)
//  2. Local config (.llmd/config.yaml)
//  3. Global config (~/.llmd/config.yaml)
//  4. Error if required values not set
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/paths"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration values.
type Config struct {
	// Author is the default author for write operations.
	Author string `yaml:"author,omitempty"`

	// Output is the default output format (markdown, json).
	Output string `yaml:"output,omitempty"`
}

// Load loads config from global and local files, merging them.
// Local values override global values. Returns an empty config if
// no config files exist (not an error).
func Load() (*Config, error) {
	cfg := &Config{}

	// Load global first
	globalPath, err := GlobalPath()
	if err == nil {
		if global, err := loadFile(globalPath); err == nil {
			cfg = global
		}
	}

	// Load local (overrides global)
	localPath := LocalPath()
	if local, err := loadFile(localPath); err == nil {
		if local.Author != "" {
			cfg.Author = local.Author
		}
		if local.Output != "" {
			cfg.Output = local.Output
		}
	}

	return cfg, nil
}

// GlobalPath returns the global config file path (~/.llmd/config.yaml).
func GlobalPath() (string, error) {
	dir, err := paths.GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LocalPath returns the local config file path (.llmd/config.yaml).
func LocalPath() string {
	return filepath.Join(".llmd", "config.yaml")
}

// Set writes a key-value pair to the specified config file.
// Creates the file and parent directories if they don't exist.
func Set(path, key, value string) error {
	// Load existing config or create empty
	cfg, err := loadFile(path)
	if err != nil {
		cfg = &Config{}
	}

	// Set the value
	switch key {
	case "author":
		cfg.Author = value
	case "output":
		cfg.Output = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write config
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Get retrieves a specific config value by key.
func (c *Config) Get(key string) (string, bool) {
	switch key {
	case "author":
		return c.Author, c.Author != ""
	case "output":
		return c.Output, c.Output != ""
	default:
		return "", false
	}
}

// loadFile loads a config from a YAML file.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}
