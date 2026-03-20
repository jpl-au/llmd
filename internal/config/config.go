// Package config reads and writes llmd configuration files.
//
// Config uses nested YAML. Two file locations are checked: local
// (.llmd/config.yaml) and global (~/.llmd/config.yaml). If a local
// file exists it is used; otherwise the global file is used. There
// is no merge - one file wins entirely.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration values. Fields map directly to
// the nested YAML structure in config.yaml.
type Config struct {
	Author  string                   `yaml:"author,omitempty"`
	Server  ServerConfig             `yaml:"server,omitempty"`
	Log     LogConfig                `yaml:"log,omitempty"`
	Limits  LimitConfig              `yaml:"limits,omitempty"`
	Webhook map[string]WebhookConfig `yaml:"webhook,omitempty"`
}

// WebhookConfig holds settings for a single webhook endpoint.
type WebhookConfig struct {
	URL string `yaml:"url"`
	Key string `yaml:"key,omitempty"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr string `yaml:"addr,omitempty"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `yaml:"level,omitempty"`
	Format string `yaml:"format,omitempty"`
}

// LimitConfig holds validation thresholds.
type LimitConfig struct {
	PathLength  int `yaml:"path_length,omitempty"`
	ContentSize int `yaml:"content_size,omitempty"`
}

// Defaults returns a Config with sensible default values. Fields
// that have no universal default (author, log level/format) are
// left as zero values - consumers apply contextual defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr: "localhost:5563",
		},
		Limits: LimitConfig{
			PathLength:  1024,
			ContentSize: 10 * 1024 * 1024, // 10 MB
		},
	}
}

// Load reads configuration from disk. It checks local
// (.llmd/config.yaml) first; if that exists, it is used. Otherwise
// the global file (~/.llmd/config.yaml) is used. Defaults are
// applied for any values not present in the file.
func Load() (Config, error) {
	cfg := Defaults()

	local := filepath.Join(".llmd", "config.yaml")
	if data, err := os.ReadFile(local); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("local config: %w", err)
		}
		return cfg, nil
	} else if !os.IsNotExist(err) {
		return cfg, fmt.Errorf("local config: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}
	global := filepath.Join(home, ".llmd", "config.yaml")
	data, err := os.ReadFile(global)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("global config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("global config: %w", err)
	}
	return cfg, nil
}

// Save writes a single config value by dot-notation key. When
// global is true it writes to ~/.llmd/config.yaml; otherwise it
// writes to the local .llmd/config.yaml. Only the target file is
// read and rewritten - the other file is not touched.
func Save(key, value string, global bool) error {
	path := filepath.Join(".llmd", "config.yaml")
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".llmd", "config.yaml")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Load existing values from this specific file (no defaults
	// applied - we only persist what the user has set).
	var cfg Config
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("reading existing config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading existing config: %w", err)
	}

	if err := cfg.Set(key, value); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// Path returns the config file path that would be used for reads.
// Local .llmd/config.yaml takes precedence over global.
func Path() string {
	local := filepath.Join(".llmd", "config.yaml")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".llmd", "config.yaml")
}

// Keys returns all valid dot-notation config keys.
func Keys() []string {
	return []string{
		"author",
		"server.addr",
		"log.level",
		"log.format",
		"limits.path_length",
		"limits.content_size",
	}
}

// Field returns the string representation of a config value by
// dot-notation key. The second return indicates whether the field
// has a non-zero value.
func (c Config) Field(key string) (string, bool) {
	switch key {
	case "author":
		return c.Author, c.Author != ""
	case "server.addr":
		return c.Server.Addr, c.Server.Addr != ""
	case "log.level":
		return c.Log.Level, c.Log.Level != ""
	case "log.format":
		return c.Log.Format, c.Log.Format != ""
	case "limits.path_length":
		if c.Limits.PathLength == 0 {
			return "", false
		}
		return strconv.Itoa(c.Limits.PathLength), true
	case "limits.content_size":
		if c.Limits.ContentSize == 0 {
			return "", false
		}
		return strconv.Itoa(c.Limits.ContentSize), true
	default:
		return "", false
	}
}

// Set assigns a value to a config field by dot-notation key.
// Returns an error for unknown keys or invalid values.
func (c *Config) Set(key, value string) error {
	switch key {
	case "author":
		c.Author = value
	case "server.addr":
		c.Server.Addr = value
	case "log.level":
		switch value {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("log.level must be one of: debug, info, warn, error")
		}
		c.Log.Level = value
	case "log.format":
		switch value {
		case "text", "json":
		default:
			return fmt.Errorf("log.format must be one of: text, json")
		}
		c.Log.Format = value
	case "limits.path_length":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("limits.path_length must be an integer")
		}
		c.Limits.PathLength = n
	case "limits.content_size":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("limits.content_size must be an integer")
		}
		c.Limits.ContentSize = n
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}
