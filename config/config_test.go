package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestSaveIsPrivate checks that the config file is readable by its
// owner only, since it can hold server.token and webhook keys, and that
// a save replaces a file another tool left readable by everyone.
func TestSaveIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not Unix permissions on Windows")
	}
	t.Chdir(t.TempDir())
	path := filepath.Join(".llmd", "config.yaml")
	if err := os.MkdirAll(".llmd", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("author: alice\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save("server.token", "s3cret", false); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("config mode = %o, want 600", got)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Author != "alice" || cfg.Server.Token != "s3cret" {
		t.Errorf("Load() author %q token %q, want alice and s3cret", cfg.Author, cfg.Server.Token)
	}
	leftovers, _ := filepath.Glob(filepath.Join(".llmd", ".config.yaml.*"))
	if len(leftovers) > 0 {
		t.Errorf("Save() left temporary files %v", leftovers)
	}
}

// TestSet covers the keys whose values are checked: a good value is
// stored, a bad one is refused in the words `llmd config` prints.
func TestSet(t *testing.T) {
	tests := []struct {
		key, value string
		wantErr    string
	}{
		{key: "server.read_timeout", value: "30s"},
		{key: "server.read_timeout", value: "soon", wantErr: "server.read_timeout must be a duration such as 30s or 2m."},
		{key: "server.shutdown_timeout", value: "0s", wantErr: "server.shutdown_timeout must be a duration such as 30s or 2m."},
		{key: "git.timeout", value: "5m"},
		{key: "database.busy_timeout", value: "-1s", wantErr: "database.busy_timeout must be a duration such as 30s or 2m."},
		{key: "database.cache_size", value: "0", wantErr: "database.cache_size must be a positive whole number."},
		{key: "database.mmap_size", value: "0"},
		{key: "database.mmap_size", value: "-1", wantErr: "database.mmap_size must be 0 or a positive whole number."},
		{key: "limits.path_length", value: "0", wantErr: "limits.path_length must be a positive whole number."},
		{key: "limits.content_size", value: "1048576"},
		{key: "agent.worktree", value: "false"},
		{key: "agent.worktree", value: "true"},
		{key: "agent.worktree", value: "maybe", wantErr: "agent.worktree must be true or false."},
		{key: "no.such.key", value: "x", wantErr: "there is no config key no.such.key. Run llmd config to list them."},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			var c Config
			err := c.Set(tt.key, tt.value)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Set(%s, %s) error = %v, want nil", tt.key, tt.value, err)
			case tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr):
				t.Fatalf("Set(%s, %s) error = %v, want %q", tt.key, tt.value, err, tt.wantErr)
			}
			if tt.wantErr == "" {
				if got, _ := c.Field(tt.key); got != tt.value {
					t.Errorf("Field(%s) after Set = %q, want %q", tt.key, got, tt.value)
				}
			}
		})
	}
}

// TestAgentWorktree checks that spawned agents get their own worktree
// unless agent.worktree turns it off.
func TestAgentWorktree(t *testing.T) {
	tests := []struct {
		name     string
		worktree *bool
		want     bool
	}{
		{"unset", nil, true},
		{"on", new(true), true},
		{"off", new(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			c.Agent.Worktree = tt.worktree
			if got := c.AgentWorktree(); got != tt.want {
				t.Errorf("AgentWorktree() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidate checks that every unusable value in a loaded config is
// reported, and that the accessors fall back to the defaults for them.
func TestValidate(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err != nil {
		t.Fatalf("Defaults().Validate() = %v, want nil", err)
	}

	c.Server.ReadTimeout = "soon"
	c.Limits.PathLength = 0
	c.Database.CacheSize = -5
	c.Webhook = map[string]WebhookConfig{"ci": {URL: "ftp://example.com", Timeout: "later", Retries: -1}}

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want the problems listed")
	}
	for _, want := range []string{
		"server.read_timeout must be a duration such as 30s or 2m.",
		"limits.path_length must be a positive whole number.",
		"database.cache_size must be a positive whole number.",
		"webhook.ci.url must be an http or https URL.",
		"webhook.ci.timeout must be a duration such as 10s.",
		"webhook.ci.retries must be 0 or more.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() = %q, want it to report %q", err, want)
		}
	}

	if got := c.DurationSetting("server.read_timeout"); got != time.Minute {
		t.Errorf("DurationSetting(server.read_timeout) = %s, want the 1m default", got)
	}
	if got := c.CountSetting("limits.path_length", 1); got != 1024 {
		t.Errorf("CountSetting(limits.path_length) = %d, want the 1024 default", got)
	}
	if got := c.CountSetting("database.cache_size", 1); got != 64 {
		t.Errorf("CountSetting(database.cache_size) = %d, want the 64 default", got)
	}
}
