// Package config provides reading and writing of llmd configuration.
// Supports both global (~/.llmd/config.yaml) and local (.llmd/config.yaml).
// Reading: uses local if it exists, otherwise global.
// Writing: defaults to global, use --local for local.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	// ErrNoConfigPath is returned when the config path cannot be determined.
	ErrNoConfigPath = errors.New("cannot determine config path")
	// ErrUnknownKey is returned when getting/setting an unknown config key.
	ErrUnknownKey = errors.New("unknown config key")
	// ErrInvalidValue is returned when a config value is invalid.
	ErrInvalidValue = errors.New("invalid config value")
)

// Scope represents the configuration scope (global or local).
type Scope int

const (
	// ScopeGlobal is user-wide config in ~/.llmd/config.yaml (default)
	ScopeGlobal Scope = iota
	// ScopeLocal is repository-specific config in .llmd/config.yaml
	ScopeLocal
)

// Author represents the author metadata stored in the repository config.
type Author struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
}

// Sync holds sync-related configuration options.
type Sync struct {
	Files *bool `yaml:"files,omitempty"`
}

// Limits holds size limit configuration options.
type Limits struct {
	MaxPath       *int   `yaml:"max_path,omitempty"`
	MaxContent    *int64 `yaml:"max_content,omitempty"`
	MaxLineLength *int   `yaml:"max_line_length,omitempty"`
}

// Default limits applied when not configured.
const (
	DefaultMaxPath       = 1024
	DefaultMaxContent    = 100 * 1024 * 1024 // 100 MB
	DefaultMaxLineLength = 10 * 1024 * 1024  // 10 MB
)

// Validation bounds for configuration values.
const (
	MinMaxPath       = 1
	MaxMaxPath       = 65536 // 64 KB - reasonable upper bound for paths
	MinMaxContent    = 1
	MaxMaxContent    = 10 * 1024 * 1024 * 1024 // 10 GB - reasonable upper bound
	MinMaxLineLength = 1
	MaxMaxLineLength = 1024 * 1024 * 1024 // 1 GB
)

// Config contains configuration for llmd.
type Config struct {
	Author Author `yaml:"author,omitempty"`
	Sync   Sync   `yaml:"sync,omitempty"`
	Limits Limits `yaml:"limits,omitempty"`

	// path is the file this config was loaded from (for Save)
	path  string
	scope Scope
}

// Validate checks that all configured values are within acceptable bounds.
// Returns nil if all values are valid or not set (defaults will be used).
func (c *Config) Validate() error {
	if c.Limits.MaxPath != nil {
		v := *c.Limits.MaxPath
		if v < MinMaxPath || v > MaxMaxPath {
			return fmt.Errorf("%w: max_path must be between %d and %d, got %d",
				ErrInvalidValue, MinMaxPath, MaxMaxPath, v)
		}
	}
	if c.Limits.MaxContent != nil {
		v := *c.Limits.MaxContent
		if v < MinMaxContent || v > MaxMaxContent {
			return fmt.Errorf("%w: max_content must be between %d and %d, got %d",
				ErrInvalidValue, MinMaxContent, MaxMaxContent, v)
		}
	}
	if c.Limits.MaxLineLength != nil {
		v := *c.Limits.MaxLineLength
		if v < MinMaxLineLength || v > MaxMaxLineLength {
			return fmt.Errorf("%w: max_line_length must be between %d and %d, got %d",
				ErrInvalidValue, MinMaxLineLength, MaxMaxLineLength, v)
		}
	}
	return nil
}

// SyncFiles returns whether file syncing is enabled (defaults to false)
func (c *Config) SyncFiles() bool {
	if c.Sync.Files == nil {
		return false
	}
	return *c.Sync.Files
}

// MaxPath returns the maximum path length in bytes (defaults to 1024).
func (c *Config) MaxPath() int {
	if c.Limits.MaxPath == nil {
		return DefaultMaxPath
	}
	return *c.Limits.MaxPath
}

// MaxContent returns the maximum content size in bytes (defaults to 100 MB).
func (c *Config) MaxContent() int64 {
	if c.Limits.MaxContent == nil {
		return DefaultMaxContent
	}
	return *c.Limits.MaxContent
}

// MaxLineLength returns the maximum line length for scanning (defaults to 10 MB).
// Affects cat and grep operations on documents with very long lines
// (e.g., minified JS/CSS, large JSON, base64 blobs).
func (c *Config) MaxLineLength() int {
	if c.Limits.MaxLineLength == nil {
		return DefaultMaxLineLength
	}
	return *c.Limits.MaxLineLength
}

// LocalPath returns the path to the local (repository) config file.
func LocalPath() string {
	return filepath.Join(".llmd", "config.yaml")
}

// GlobalPath returns the path to the global (user) config file: ~/.llmd/config.yaml
func GlobalPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".llmd", "config.yaml")
}

// Path returns the local config path (for backwards compatibility).
func Path() string {
	return LocalPath()
}

// Load reads configuration: uses local if it exists, otherwise global.
func Load() (*Config, error) {
	// Check if local config exists
	if _, err := os.Stat(LocalPath()); err == nil {
		return LoadScope(ScopeLocal)
	}
	// Fall back to global
	return LoadScope(ScopeGlobal)
}

// LoadScope reads configuration from a specific scope.
func LoadScope(scope Scope) (*Config, error) {
	path := pathForScope(scope)
	if path == "" {
		return &Config{scope: scope}, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{path: path, scope: scope}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("malformed config file %s: %w\n\nTo fix: edit the file to correct the YAML syntax, or delete it to use defaults", path, err)
	}
	cfg.path = path
	cfg.scope = scope

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config file %s: %w", path, err)
	}
	return &cfg, nil
}

// Scope returns which scope this config was loaded from.
func (c *Config) Scope() Scope {
	return c.scope
}

// Save writes the configuration to its original location.
func (c *Config) Save() error {
	if c.path == "" {
		c.path = pathForScope(c.scope)
	}
	if c.path == "" {
		return ErrNoConfigPath
	}
	return c.saveToPath(c.path)
}

// SaveScope writes the configuration to the specified scope.
func (c *Config) SaveScope(scope Scope) error {
	path := pathForScope(scope)
	if path == "" {
		return ErrNoConfigPath
	}
	return c.saveToPath(path)
}

// saveToPath writes configuration to a specific filesystem path.
// Creates parent directories as needed with mode 0755.
func (c *Config) saveToPath(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// pathForScope returns the filesystem path for a given scope.
func pathForScope(scope Scope) string {
	switch scope {
	case ScopeLocal:
		return LocalPath()
	case ScopeGlobal:
		return GlobalPath()
	default:
		return ""
	}
}
