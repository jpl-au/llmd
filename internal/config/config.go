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
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration values. Fields map directly to
// the nested YAML structure in config.yaml.
type Config struct {
	Author  string                   `yaml:"author,omitempty"`
	Server  ServerConfig             `yaml:"server,omitempty"`
	Log     LogConfig                `yaml:"log,omitempty"`
	Limits  LimitConfig              `yaml:"limits,omitempty"`
	Task    TaskConfig               `yaml:"task,omitempty"`
	Queue   QueueConfig              `yaml:"queue,omitempty"`
	Webhook map[string]WebhookConfig `yaml:"webhook,omitempty"`
}

// QueueConfig holds message queue settings.
type QueueConfig struct {
	// Retention is how long queue messages are kept before vacuum
	// removes them: a number of days such as 30d, a duration such as
	// 12h, or 0 to keep every message. See Retention.
	Retention string `yaml:"retention,omitempty"`
}

// Retention parses a retention setting: a number of days such as 30d,
// a duration such as 12h or 90m, or 0 (and "") for no limit.
func Retention(s string) (time.Duration, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil && n >= 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid retention %q: use a number of days (30d), a duration (12h), or 0 to keep everything", s)
	}
	return d, nil
}

// TaskConfig holds task board settings.
type TaskConfig struct {
	// Order is how tasks are ordered within a column, for the board,
	// review and task next: "priority" (priority 1 first, then board
	// position; the default) or "position" (board position only).
	Order string `yaml:"order,omitempty"`
}

// WebhookConfig holds settings for a single webhook endpoint.
type WebhookConfig struct {
	URL     string `yaml:"url"`
	Key     string `yaml:"key,omitempty"`     // sent as Authorization: Bearer <key>
	Retries int    `yaml:"retries,omitempty"` // retries after the first attempt (default 2)
	Timeout string `yaml:"timeout,omitempty"` // per request, a duration such as 10s (default 10s)
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr           string `yaml:"addr,omitempty"`
	Token          string `yaml:"token,omitempty"`           // when set, every request must carry Authorization: Bearer <token>
	WebhookWorkers int    `yaml:"webhook_workers,omitempty"` // goroutines delivering webhooks (default 4)
	WebhookQueue   int    `yaml:"webhook_queue,omitempty"`   // events that may wait for a worker (default 256)
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
		Task: TaskConfig{
			Order: "priority",
		},
		Queue: QueueConfig{
			Retention: "30d",
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

// Keys returns all valid dot-notation config keys.
func Keys() []string {
	return []string{
		"author",
		"server.addr",
		"server.token",
		"server.webhook_workers",
		"server.webhook_queue",
		"log.level",
		"log.format",
		"limits.path_length",
		"limits.content_size",
		"task.order",
		"queue.retention",
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
	case "server.token":
		return c.Server.Token, c.Server.Token != ""
	case "server.webhook_workers":
		return strconv.Itoa(c.Server.WebhookWorkers), c.Server.WebhookWorkers != 0
	case "server.webhook_queue":
		return strconv.Itoa(c.Server.WebhookQueue), c.Server.WebhookQueue != 0
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
	case "task.order":
		return c.Task.Order, c.Task.Order != ""
	case "queue.retention":
		return c.Queue.Retention, c.Queue.Retention != ""
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
	case "server.token":
		c.Server.Token = value
	case "server.webhook_workers":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("server.webhook_workers must be a positive integer")
		}
		c.Server.WebhookWorkers = n
	case "server.webhook_queue":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("server.webhook_queue must be a positive integer")
		}
		c.Server.WebhookQueue = n
	case "log.level":
		switch value {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("log.level must be one of: debug, info, warn, error")
		}
		c.Log.Level = value
	case "task.order":
		switch value {
		case "priority", "position":
		default:
			return fmt.Errorf("task.order must be priority or position")
		}
		c.Task.Order = value
	case "queue.retention":
		if _, err := Retention(value); err != nil {
			return fmt.Errorf("queue.retention must be a number of days (30d), a duration (12h), or 0 to keep every message")
		}
		c.Queue.Retention = value
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
