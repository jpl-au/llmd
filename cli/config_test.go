package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// metaConfigWorkdir sets up a store directory as the working directory
// and points HOME at an empty temporary directory so neither the local
// nor the global config of the developer running the tests leaks in.
func metaConfigWorkdir(t *testing.T, author string) (sdk.Context, string) {
	t.Helper()
	host.TestHome(t)
	return metaWorkdir(t, author)
}

// TestConfig covers show-all, show-key and set-key for the local and
// global files, local precedence over global, and the error cases.
// Each case runs in a fresh working directory with an empty HOME.
func TestConfig(t *testing.T) {
	tests := []struct {
		name       string
		seed       func(t *testing.T, c sdk.Context)
		args       []string
		wantText   []string
		wantAbsent []string
		wantExact  *string // exact response text, when set
		wantFile   string  // relative to the working directory, or $HOME
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:       "show all defaults",
			args:       nil,
			wantText:   []string{"server.addr=localhost:5563", "limits.path_length=1024", "limits.content_size=10485760"},
			wantAbsent: []string{"author="},
		},
		{
			name:      "show unset key is empty",
			args:      []string{"author"},
			wantExact: new(string),
		},
		{
			name:      "show default key",
			args:      []string{"server.addr"},
			wantExact: new("localhost:5563"),
		},
		{
			name:      "show unknown key is empty",
			args:      []string{"nope.key"},
			wantExact: new(string),
		},
		{
			name:     "set writes the local file",
			args:     []string{"author", "Alice"},
			wantFile: filepath.Join(".llmd", "config.yaml"),
		},
		{
			name: "set then show key",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "author", "Alice")
			},
			args:      []string{"author"},
			wantExact: new("Alice"),
		},
		{
			name: "set then show all",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "author", "Alice")
				run(t, c, "config", nil, "log.level", "debug")
			},
			args:     nil,
			wantText: []string{"author=Alice", "log.level=debug", "server.addr=localhost:5563"},
		},
		{
			name: "set nested key keeps earlier values",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "author", "Alice")
				run(t, c, "config", nil, "limits.path_length", "2048")
			},
			args:     nil,
			wantText: []string{"author=Alice", "limits.path_length=2048"},
		},
		{
			name: "set server token then show key",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "server.token", "s3cret")
			},
			args:      []string{"server.token"},
			wantExact: new("s3cret"),
		},
		{
			name: "set webhook workers then show key",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "server.webhook_workers", "8")
			},
			args:      []string{"server.webhook_workers"},
			wantExact: new("8"),
		},
		{
			name:       "webhook queue must be positive",
			args:       []string{"server.webhook_queue", "0"},
			wantAnyErr: true,
		},
		{
			name:     "set global writes under HOME",
			args:     []string{"--global", "author", "Bob"},
			wantFile: "$HOME/.llmd/config.yaml",
		},
		{
			name: "global is read when no local file exists",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "--global", "author", "Bob")
			},
			args:      []string{"author"},
			wantExact: new("Bob"),
		},
		{
			name: "local file wins over global",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "config", nil, "--global", "author", "Bob")
				run(t, c, "config", nil, "author", "Alice")
			},
			args:      []string{"author"},
			wantExact: new("Alice"),
		},
		{
			name:       "unknown key cannot be set",
			args:       []string{"nope.key", "x"},
			wantAnyErr: true,
		},
		{
			name:       "invalid integer value",
			args:       []string{"limits.path_length", "lots"},
			wantAnyErr: true,
		},
		{
			name:       "too many arguments",
			args:       []string{"author", "Alice", "extra"},
			wantAnyErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, dir := metaConfigWorkdir(t, "alice")
			if tt.seed != nil {
				tt.seed(t, c)
			}

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "config", nil, tt.args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("config %v error = nil, want an error", tt.args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("config %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "config", nil, tt.args...)
			got := text(resp)
			if tt.wantExact != nil && got != *tt.wantExact {
				t.Errorf("config %v = %q, want %q", tt.args, got, *tt.wantExact)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("config %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("config %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if tt.wantFile != "" {
				path := strings.Replace(tt.wantFile, "$HOME", os.Getenv("HOME"), 1)
				if !filepath.IsAbs(path) {
					path = filepath.Join(dir, path)
				}
				if _, err := os.Stat(path); err != nil {
					t.Errorf("after config %v, %s: %v", tt.args, path, err)
				}
			}
		})
	}
}

// TestConfigGit covers the git subcommand: listing the whitelist,
// allowing and denying patterns (including idempotence), and the
// error cases. Each case runs in a fresh store directory; seedDefault
// writes the whitelist that init creates.
func TestConfigGit(t *testing.T) {
	seedDefault := func(t *testing.T, c sdk.Context) {
		if err := config.InitGitignoreIn(config.StoreDir); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name       string
		author     string
		seed       func(t *testing.T, c sdk.Context)
		args       []string
		wantText   []string
		wantAbsent []string
		wantExact  *string
		wantRules  []string // exact rules listed afterwards, when set
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:      "list without a gitignore is empty",
			author:    "alice",
			args:      []string{"git", "ls"},
			wantExact: new(string),
		},
		{
			name:      "list the default whitelist",
			author:    "alice",
			seed:      seedDefault,
			args:      []string{"git", "ls"},
			wantText:  []string{"*", "!llmd.db", "!.gitignore"},
			wantRules: []string{"*", "!llmd.db", "!.gitignore"},
		},
		{
			name:     "bare git lists",
			author:   "alice",
			seed:     seedDefault,
			args:     []string{"git"},
			wantText: []string{"!llmd.db"},
		},
		{
			name:     "list alias",
			author:   "alice",
			seed:     seedDefault,
			args:     []string{"git", "list"},
			wantText: []string{"!llmd.db"},
		},
		{
			name:      "allow appends a whitelist entry",
			author:    "alice",
			seed:      seedDefault,
			args:      []string{"git", "allow", "reports/"},
			wantExact: new("Allowed reports/ in .llmd/.gitignore"),
			wantRules: []string{"*", "!llmd.db", "!.gitignore", "!reports/"},
		},
		{
			name:      "allow without a gitignore creates one",
			author:    "alice",
			args:      []string{"git", "allow", "reports/"},
			wantRules: []string{"!reports/"},
		},
		{
			name:   "allow twice is idempotent",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context) {
				seedDefault(t, c)
				run(t, c, "config", nil, "git", "allow", "reports/")
			},
			args:      []string{"git", "allow", "reports/"},
			wantRules: []string{"*", "!llmd.db", "!.gitignore", "!reports/"},
		},
		{
			name:   "deny removes the entry",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context) {
				seedDefault(t, c)
				run(t, c, "config", nil, "git", "allow", "reports/")
			},
			args:      []string{"git", "deny", "reports/"},
			wantExact: new("Denied reports/ in .llmd/.gitignore"),
			wantRules: []string{"*", "!llmd.db", "!.gitignore"},
		},
		{
			name:      "deny a default entry",
			author:    "alice",
			seed:      seedDefault,
			args:      []string{"git", "deny", "llmd.db"},
			wantRules: []string{"*", "!.gitignore"},
		},
		{
			name:       "deny unknown pattern",
			author:     "alice",
			seed:       seedDefault,
			args:       []string{"git", "deny", "nope/"},
			wantAnyErr: true,
		},
		{
			name:    "allow without pattern",
			author:  "alice",
			args:    []string{"git", "allow"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "deny without pattern",
			author:  "alice",
			args:    []string{"git", "deny"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:       "unknown subcommand",
			author:     "alice",
			args:       []string{"git", "bogus"},
			wantAnyErr: true,
		},
		{
			// config does not require an author; the whitelist is not
			// attributed.
			name:      "allow without author",
			author:    "",
			seed:      seedDefault,
			args:      []string{"git", "allow", "reports/"},
			wantRules: []string{"*", "!llmd.db", "!.gitignore", "!reports/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed, _ := metaConfigWorkdir(t, "alice")
			if tt.seed != nil {
				tt.seed(t, seed)
			}
			c := seed
			c.Author = tt.author

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "config", nil, tt.args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("config %v error = nil, want an error", tt.args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("config %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "config", nil, tt.args...)
			got := text(resp)
			if tt.wantExact != nil && got != *tt.wantExact {
				t.Errorf("config %v = %q, want %q", tt.args, got, *tt.wantExact)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("config %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("config %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if tt.wantRules != nil {
				ls := run(t, seed, "config", nil, "git", "ls")
				rules, ok := data(ls).([]string)
				if !ok {
					t.Fatalf("config git ls Data = %T, want []string", data(ls))
				}
				if strings.Join(rules, "\n") != strings.Join(tt.wantRules, "\n") {
					t.Errorf("config git ls after %v = %v, want %v", tt.args, rules, tt.wantRules)
				}
				if text(ls) != strings.Join(tt.wantRules, "\n") {
					t.Errorf("config git ls text after %v = %q, want %q", tt.args, text(ls), strings.Join(tt.wantRules, "\n"))
				}
			}
		})
	}
}

// TestConfigTaskOrder covers the task.order key: listed with its
// default, settable to position, and refused for anything else.
func TestConfigTaskOrder(t *testing.T) {
	c := taskWorkdirCLI(t, "alice")
	if got := text(run(t, c, "config", nil)); !strings.Contains(got, "task.order=priority") {
		t.Errorf("config = %q, want to list task.order=priority", got)
	}
	run(t, c, "config", nil, "task.order", "position")
	if got := text(run(t, c, "config", nil, "task.order")); !strings.Contains(got, "position") {
		t.Errorf("config task.order = %q, want position", got)
	}
	if err := runErr(t, c, "config", nil, "task.order", "random"); err == nil || !strings.Contains(err.Error(), "priority or position") {
		t.Errorf("config task.order random error = %v, want a message naming the valid values", err)
	}
}

// TestConfigQueueRetention covers the queue.retention key: listed with
// its default, settable to days, a duration or 0, and refused otherwise.
func TestConfigQueueRetention(t *testing.T) {
	c := taskWorkdirCLI(t, "alice")
	if got := text(run(t, c, "config", nil)); !strings.Contains(got, "queue.retention=30d") {
		t.Errorf("config = %q, want to list queue.retention=30d", got)
	}
	for _, v := range []string{"7d", "12h", "0"} {
		run(t, c, "config", nil, "queue.retention", v)
		if got := text(run(t, c, "config", nil, "queue.retention")); !strings.Contains(got, v) {
			t.Errorf("config queue.retention = %q, want %s", got, v)
		}
	}
	if err := runErr(t, c, "config", nil, "queue.retention", "soon"); err == nil || !strings.Contains(err.Error(), "30d") {
		t.Errorf("config queue.retention soon error = %v, want a message showing the accepted forms", err)
	}
}
