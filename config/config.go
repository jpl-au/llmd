// Package config reads and writes llmd configuration files.
//
// Config uses nested YAML. Two file locations are checked: local
// (.llmd/config.yaml) and global (~/.llmd/config.yaml). If a local
// file exists it is used; otherwise the global file is used. There
// is no merge - one file wins entirely.
package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration values. Fields map directly to
// the nested YAML structure in config.yaml.
type Config struct {
	Author   string                   `yaml:"author,omitempty"`
	Server   ServerConfig             `yaml:"server,omitempty"`
	Log      LogConfig                `yaml:"log,omitempty"`
	Limits   LimitConfig              `yaml:"limits,omitempty"`
	Task     TaskConfig               `yaml:"task,omitempty"`
	Queue    QueueConfig              `yaml:"queue,omitempty"`
	Hook     HookConfig               `yaml:"hook,omitempty"`
	Webhook  map[string]WebhookConfig `yaml:"webhook,omitempty"`
	Git      GitConfig                `yaml:"git,omitempty"`
	Database DatabaseConfig           `yaml:"database,omitempty"`
	Display  DisplayConfig            `yaml:"display,omitempty"`
	Agent    AgentConfig              `yaml:"agent,omitempty"`
}

// AgentConfig holds settings for spawned agents.
type AgentConfig struct {
	// Worktree is whether a spawned agent works in its own git
	// worktree (true, the default) or in the main checkout.
	Worktree *bool `yaml:"worktree,omitempty"`
	// Harness names the harness that runs agents on this computer, in
	// place of the one the store names for each agent. Empty leaves the
	// choice to the store.
	Harness string `yaml:"harness,omitempty"`
}

// DisplayConfig holds settings for output at a terminal.
type DisplayConfig struct {
	// Theme is the colour scheme rendered markdown uses: "dark" (the
	// default) for a dark terminal background, "light" for a light
	// one. Piped output is never coloured.
	Theme string `yaml:"theme,omitempty"`
}

// DefaultAddr is the address llmd serve listens on unless server.addr
// says otherwise.
const DefaultAddr = "localhost:5563"

// GitConfig holds settings for the git commands tasks and agents run.
type GitConfig struct {
	// Timeout is how long one git command may run before it is
	// stopped, a duration such as 2m.
	Timeout string `yaml:"timeout,omitempty"`
}

// DatabaseConfig holds SQLite settings for llmd.db and work.db. They
// apply to this process's connections; nothing is stored in the files.
type DatabaseConfig struct {
	// BusyTimeout is how long a write waits for another process's
	// write to finish before failing, a duration such as 5s. Raise it
	// when several agents write to one store at once.
	BusyTimeout string `yaml:"busy_timeout,omitempty"`
	// CacheSize is the most page cache, in MB, each connection may
	// hold. Memory is used only as pages are read.
	CacheSize int `yaml:"cache_size,omitempty"`
	// MMapSize is how much of each file, in MB, is memory-mapped; 0
	// turns memory-mapping off.
	MMapSize *int `yaml:"mmap_size,omitempty"`
	// Synchronous is how often writes are flushed to disk: "full",
	// "normal" (the default) or "off". Off is fastest, but a power
	// failure or operating system crash can then corrupt the database.
	Synchronous string `yaml:"synchronous,omitempty"`
}

// HookConfig holds agent hook settings.
type HookConfig struct {
	// Limit is the most pending queue messages the session-start hook
	// prints into an agent's context; older ones are counted, not
	// shown. 0 prints every message. A pointer so that an explicit 0
	// survives the round trip through the file.
	Limit *int `yaml:"limit,omitempty"`
}

// AgentWorktree reports whether spawned agents get their own git
// worktree, the default when the key is unset.
func (c Config) AgentWorktree() bool {
	if c.Agent.Worktree == nil {
		return *Defaults().Agent.Worktree
	}
	return *c.Agent.Worktree
}

// HookLimit returns the configured limit, or the default when unset.
func (c Config) HookLimit() int {
	if c.Hook.Limit == nil {
		return *Defaults().Hook.Limit
	}
	return *c.Hook.Limit
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

// ServerConfig holds HTTP server settings. Durations are strings such
// as 10s or 2m.
type ServerConfig struct {
	Addr              string `yaml:"addr,omitempty"`
	Token             string `yaml:"token,omitempty"`               // when set, every request must carry Authorization: Bearer <token>
	WebhookWorkers    int    `yaml:"webhook_workers,omitempty"`     // goroutines delivering webhooks (default 4)
	WebhookQueue      int    `yaml:"webhook_queue,omitempty"`       // events that may wait for a worker (default 256)
	ReadHeaderTimeout string `yaml:"read_header_timeout,omitempty"` // time a client has to send its headers
	ReadTimeout       string `yaml:"read_timeout,omitempty"`        // time a client has to send the whole request
	IdleTimeout       string `yaml:"idle_timeout,omitempty"`        // how long an unused keep-alive connection stays open
	ShutdownTimeout   string `yaml:"shutdown_timeout,omitempty"`    // time requests in flight get to finish on shutdown
	WatchInterval     string `yaml:"watch_interval,omitempty"`      // how often to look for changes other processes made
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
			Addr:              DefaultAddr,
			ReadHeaderTimeout: "10s",
			ReadTimeout:       "1m",
			IdleTimeout:       "2m",
			ShutdownTimeout:   "5s",
			WatchInterval:     "1s",
		},
		Git: GitConfig{
			Timeout: "2m",
		},
		Database: DatabaseConfig{
			BusyTimeout: "5s",
			CacheSize:   64,
			MMapSize:    new(256),
			Synchronous: "normal",
		},
		Display: DisplayConfig{
			Theme: "dark",
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
		Hook: HookConfig{
			Limit: new(10),
		},
		Agent: AgentConfig{
			Worktree: new(true),
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

	// The global directory holds nothing anyone else needs. The local
	// one holds llmd.db, which is shared, so only the file is private.
	dirPerm := os.FileMode(0o755)
	if global {
		dirPerm = 0o700
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	// The file can hold server.token and webhook keys, so only its
	// owner may read it.
	return writeAtomic(path, data, 0o600)
}

// writeAtomic replaces path with data: it writes a temporary file in
// the same directory and renames it over path, so a crash or a full
// disk leaves the old file whole rather than a truncated one.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
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
		"hook.limit",
		"server.read_header_timeout",
		"server.read_timeout",
		"server.idle_timeout",
		"server.shutdown_timeout",
		"server.watch_interval",
		"git.timeout",
		"database.busy_timeout",
		"database.cache_size",
		"database.mmap_size",
		"database.synchronous",
		"display.theme",
		"agent.worktree",
		"agent.harness",
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
	case "hook.limit":
		if c.Hook.Limit == nil {
			return "", false
		}
		return strconv.Itoa(*c.Hook.Limit), true
	case "server.read_header_timeout":
		return c.Server.ReadHeaderTimeout, c.Server.ReadHeaderTimeout != ""
	case "server.read_timeout":
		return c.Server.ReadTimeout, c.Server.ReadTimeout != ""
	case "server.idle_timeout":
		return c.Server.IdleTimeout, c.Server.IdleTimeout != ""
	case "server.shutdown_timeout":
		return c.Server.ShutdownTimeout, c.Server.ShutdownTimeout != ""
	case "server.watch_interval":
		return c.Server.WatchInterval, c.Server.WatchInterval != ""
	case "git.timeout":
		return c.Git.Timeout, c.Git.Timeout != ""
	case "database.busy_timeout":
		return c.Database.BusyTimeout, c.Database.BusyTimeout != ""
	case "database.cache_size":
		return strconv.Itoa(c.Database.CacheSize), c.Database.CacheSize != 0
	case "database.mmap_size":
		if c.Database.MMapSize == nil {
			return "", false
		}
		return strconv.Itoa(*c.Database.MMapSize), true
	case "database.synchronous":
		return c.Database.Synchronous, c.Database.Synchronous != ""
	case "display.theme":
		return c.Display.Theme, c.Display.Theme != ""
	case "agent.worktree":
		if c.Agent.Worktree == nil {
			return "", false
		}
		return strconv.FormatBool(*c.Agent.Worktree), true
	case "agent.harness":
		return c.Agent.Harness, c.Agent.Harness != ""
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
		return setCount(&c.Server.WebhookWorkers, key, value, 1)
	case "server.webhook_queue":
		return setCount(&c.Server.WebhookQueue, key, value, 1)
	case "log.level":
		if !slices.Contains([]string{"debug", "info", "warn", "error"}, value) {
			return fmt.Errorf("log.level must be debug, info, warn or error.")
		}
		c.Log.Level = value
	case "log.format":
		if value != "text" && value != "json" {
			return fmt.Errorf("log.format must be text or json.")
		}
		c.Log.Format = value
	case "task.order":
		if value != "priority" && value != "position" {
			return fmt.Errorf("task.order must be priority or position.")
		}
		c.Task.Order = value
	case "queue.retention":
		if _, err := Retention(value); err != nil {
			return fmt.Errorf("queue.retention must be a number of days such as 30d, a duration such as 12h, or 0 to keep every message.")
		}
		c.Queue.Retention = value
	case "hook.limit":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("hook.limit must be a whole number of messages, or 0 to print every pending message.")
		}
		c.Hook.Limit = &n
	case "limits.path_length":
		return setCount(&c.Limits.PathLength, key, value, 1)
	case "limits.content_size":
		return setCount(&c.Limits.ContentSize, key, value, 1)
	case "server.read_header_timeout":
		return setDuration(&c.Server.ReadHeaderTimeout, key, value)
	case "server.read_timeout":
		return setDuration(&c.Server.ReadTimeout, key, value)
	case "server.idle_timeout":
		return setDuration(&c.Server.IdleTimeout, key, value)
	case "server.shutdown_timeout":
		return setDuration(&c.Server.ShutdownTimeout, key, value)
	case "server.watch_interval":
		return setDuration(&c.Server.WatchInterval, key, value)
	case "git.timeout":
		return setDuration(&c.Git.Timeout, key, value)
	case "database.busy_timeout":
		return setDuration(&c.Database.BusyTimeout, key, value)
	case "database.cache_size":
		return setCount(&c.Database.CacheSize, key, value, 1)
	case "database.mmap_size":
		var n int
		if err := setCount(&n, key, value, 0); err != nil {
			return err
		}
		c.Database.MMapSize = &n
	case "database.synchronous":
		if value != "full" && value != "normal" && value != "off" {
			return fmt.Errorf("database.synchronous must be full, normal or off.")
		}
		c.Database.Synchronous = value
	case "display.theme":
		if value != "dark" && value != "light" {
			return fmt.Errorf("display.theme must be dark or light.")
		}
		c.Display.Theme = value
	case "agent.worktree":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("agent.worktree must be true or false.")
		}
		c.Agent.Worktree = &b
	case "agent.harness":
		c.Agent.Harness = value
	default:
		return fmt.Errorf("there is no config key %s. Run llmd config to list them.", key)
	}
	return nil
}

// setCount parses a whole number of at least min into dst.
func setCount(dst *int, key, value string, min int) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < min {
		if min == 0 {
			return fmt.Errorf("%s must be 0 or a positive whole number.", key)
		}
		return fmt.Errorf("%s must be a positive whole number.", key)
	}
	*dst = n
	return nil
}

// setDuration checks value is a positive duration and stores it.
func setDuration(dst *string, key, value string) error {
	if _, err := Duration(value); err != nil {
		return fmt.Errorf("%s must be a duration such as 30s or 2m.", key)
	}
	*dst = value
	return nil
}

// Validate reports every value in c that `llmd config <key> <value>`
// would refuse, in the words it would use. Values a file cannot get
// wrong for a key it leaves out are not reported: Load fills those
// from Defaults. Callers keep running on a config with problems: the
// accessors (DurationSetting, CountSetting) use the default for any
// value Validate reports.
func (c Config) Validate() error {
	var problems []error
	for _, key := range Keys() {
		value, ok := c.Field(key)
		if !ok {
			continue
		}
		var scratch Config
		if err := scratch.Set(key, value); err != nil {
			problems = append(problems, err)
		}
	}
	// Field reports 0 as unset, but a file that sets these to 0 breaks
	// every write or read.
	for key, n := range map[string]int{
		"limits.path_length":  c.Limits.PathLength,
		"limits.content_size": c.Limits.ContentSize,
		"database.cache_size": c.Database.CacheSize,
	} {
		if n == 0 {
			problems = append(problems, fmt.Errorf("%s must be a positive whole number.", key))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Webhook)) {
		hook := c.Webhook[name]
		if u, err := url.Parse(hook.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			problems = append(problems, fmt.Errorf("webhook.%s.url must be an http or https URL.", name))
		}
		if hook.Timeout != "" {
			if _, err := Duration(hook.Timeout); err != nil {
				problems = append(problems, fmt.Errorf("webhook.%s.timeout must be a duration such as 10s.", name))
			}
		}
		if hook.Retries < 0 {
			problems = append(problems, fmt.Errorf("webhook.%s.retries must be 0 or more.", name))
		}
	}
	slices.SortFunc(problems, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	return errors.Join(problems...)
}

// DurationSetting returns the duration key holds, or the default when
// the value is missing or not a positive duration.
func (c Config) DurationSetting(key string) time.Duration {
	if v, ok := c.Field(key); ok {
		if d, err := Duration(v); err == nil {
			return d
		}
	}
	v, _ := Defaults().Field(key)
	d, _ := Duration(v)
	return d
}

// CountSetting returns the whole number key holds, or the default when
// the value is missing or below min.
func (c Config) CountSetting(key string, min int) int {
	if v, ok := c.Field(key); ok {
		if n, err := strconv.Atoi(v); err == nil && n >= min {
			return n
		}
	}
	v, _ := Defaults().Field(key)
	n, _ := strconv.Atoi(v)
	return n
}

// Duration parses a positive duration setting such as 10s, 2m or 1h.
func Duration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%q is not a positive duration", s)
	}
	return d, nil
}
