package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/host"
	"github.com/jpl-au/llmd/sdk"
)

// ruleCLI returns a context over a store that is also the working
// directory: rules live in .llmd/rules and seeding them whitelists the
// directory in .llmd/.gitignore, both relative to the cwd. The host is
// returned so tests can reach the rules file directly. The agents the
// tests name are registered, since rule set refuses one that is not.
func ruleCLI(t *testing.T, author string) (*host.Host, sdk.Context) {
	t.Helper()
	h := host.TestSetup(t, host.TestWorkdir)
	c := h.Context(t.Context(), author)
	for _, name := range []string{"claude-code", "gemini"} {
		run(t, c, "agent", nil, "add", name)
	}
	return h, c
}

// ruleShow returns the stored rule set.
func ruleShow(t *testing.T, c sdk.Context) map[string]sdk.ColumnRule {
	t.Helper()
	rs, err := c.Rules.Show()
	if err != nil {
		t.Fatalf("Rules.Show: %v", err)
	}
	return rs
}

// TestRuleList covers the listing (and its aliases, including the bare
// command): the seeded default transitions render as manual columns,
// an empty rule file gets the hint, and unknown subcommands fail.
func TestRuleList(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		emptyRules  bool // replace default.yaml with an empty map
		setup       func(t *testing.T, c sdk.Context)
		args        []string
		wantText    []string
		wantAbsent  []string
		wantData    map[string]sdk.ColumnRule // nil skips the check
		wantErrText string
	}{
		{
			name:     "defaults are manual transitions",
			args:     []string{"list"},
			wantText: []string{"up-next", "in-progress", "review", "[manual]", "success:", "failure:", "approval", "blocked"},
			wantData: map[string]sdk.ColumnRule{
				"up-next":     {Success: "in-progress", Failure: "up-next"},
				"in-progress": {Success: "review", Failure: "blocked"},
				"review":      {Success: "approval", Failure: "blocked"},
			},
		},
		{name: "bare rule lists", args: nil, wantText: []string{"in-progress", "[manual]"}},
		{name: "show alias", args: []string{"show"}, wantText: []string{"in-progress", "[manual]"}},
		{name: "ls alias", args: []string{"ls"}, wantText: []string{"in-progress", "[manual]"}},
		{name: "reads without author", noAuthor: true, args: []string{"list"}, wantText: []string{"in-progress"}},
		{
			name: "automated column shows agent and role",
			setup: func(t *testing.T, c sdk.Context) {
				run(t, c, "rule", nil, "set", "code", "--agent", "claude-code", "--role", "developer", "--success", "test", "--failure", "blocked")
			},
			args:     []string{"list"},
			wantText: []string{"code", "[claude-code, developer]", "test"},
		},
		{
			name:       "no rules configured",
			emptyRules: true,
			args:       []string{"list"},
			wantText:   []string{"No rules configured", "llmd rule set"},
			wantAbsent: []string{"[manual]"},
		},
		{name: "unknown subcommand", args: []string{"bogus"}, wantErrText: "unknown subcommand"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			author := "alice"
			if tt.noAuthor {
				author = ""
			}
			h, c := ruleCLI(t, author)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			if tt.emptyRules {
				// Seed first so the file exists, then empty it.
				ruleShow(t, c)
				p := filepath.Join(h.Dir(), ".llmd", "rules", "default.yaml")
				if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if tt.wantErrText != "" {
				err := runErr(t, c, "rule", nil, tt.args...)
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("rule %v error = %v, want to contain %q", tt.args, err, tt.wantErrText)
				}
				return
			}
			resp := run(t, c, "rule", nil, tt.args...)
			taskWantText(t, "rule", tt.args, text(resp), tt.wantText, tt.wantAbsent)
			if tt.wantData != nil {
				got, ok := data(resp).(map[string]sdk.ColumnRule)
				if !ok {
					t.Fatalf("rule %v Data = %T, want map[string]sdk.ColumnRule", tt.args, data(resp))
				}
				if len(got) != len(tt.wantData) {
					t.Errorf("rule %v Data = %v, want %v", tt.args, got, tt.wantData)
				}
				for col, want := range tt.wantData {
					if got[col] != want {
						t.Errorf("rule %v Data[%s] = %+v, want %+v", tt.args, col, got[col], want)
					}
				}
			}
		})
	}
}

// TestRuleSet covers every flag, the merge with an existing rule (only
// the flags given change), persistence to disk, and the errors.
func TestRuleSet(t *testing.T) {
	tests := []struct {
		name       string
		noAuthor   bool
		setup      func(t *testing.T, c sdk.Context)
		args       []string
		wantText   []string
		wantAbsent []string
		wantRule   sdk.ColumnRule
		wantErr    error
	}{
		{
			name:     "all flags on a new column",
			args:     []string{"set", "code", "--agent", "claude-code", "--role", "developer", "--success", "test", "--failure", "blocked", "--resume"},
			wantText: []string{"code", "[claude-code, developer, resume]", "success:", "test", "failure:", "blocked"},
			wantRule: sdk.ColumnRule{Agent: "claude-code", Role: "developer", Success: "test", Failure: "blocked", Resume: true},
		},
		{
			// The rule would do nothing until a task entered the
			// column, then fail with only a log line.
			name:    "an agent that is not registered is refused",
			args:    []string{"set", "code", "--agent", "nosuchagent"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:       "transitions only is a manual column",
			args:       []string{"set", "code", "--success", "test", "--failure", "blocked"},
			wantText:   []string{"code", "[manual]", "test", "blocked"},
			wantAbsent: []string{"resume"},
			wantRule:   sdk.ColumnRule{Success: "test", Failure: "blocked"},
		},
		{
			name:     "agent without role",
			args:     []string{"set", "code", "--agent", "claude-code"},
			wantText: []string{"[claude-code, ]"},
			wantRule: sdk.ColumnRule{Agent: "claude-code"},
		},
		{
			// Only the flags given change; the default transitions survive.
			name:     "merges with the existing rule",
			args:     []string{"set", "in-progress", "--agent", "gemini", "--role", "auditor"},
			wantText: []string{"in-progress", "[gemini, auditor]", "review", "blocked"},
			wantRule: sdk.ColumnRule{Agent: "gemini", Role: "auditor", Success: "review", Failure: "blocked"},
		},
		{
			name: "changing the agent keeps the role and transitions",
			setup: func(t *testing.T, c sdk.Context) {
				run(t, c, "rule", nil, "set", "code", "--agent", "claude-code", "--role", "developer", "--success", "test", "--failure", "blocked", "--resume")
			},
			args:     []string{"set", "code", "--agent", "gemini"},
			wantText: []string{"[gemini, developer, resume]"},
			wantRule: sdk.ColumnRule{Agent: "gemini", Role: "developer", Success: "test", Failure: "blocked", Resume: true},
		},
		{
			name:     "no flags writes an empty rule",
			args:     []string{"set", "code"},
			wantText: []string{"code", "[manual]"},
			wantRule: sdk.ColumnRule{},
		},
		{
			name:     "works without an author",
			noAuthor: true,
			args:     []string{"set", "code", "--success", "test"},
			wantText: []string{"code"},
			wantRule: sdk.ColumnRule{Success: "test"},
		},
		{name: "no column", args: []string{"set"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", args: []string{"set", "code", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "flag without value", args: []string{"set", "code", "--agent"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			author := "alice"
			if tt.noAuthor {
				author = ""
			}
			h, c := ruleCLI(t, author)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			before := ruleShow(t, c)

			if tt.wantErr != nil {
				if err := runErr(t, c, "rule", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("rule %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				if after := ruleShow(t, c); len(after) != len(before) {
					t.Errorf("rules after failed set = %v, want unchanged %v", after, before)
				}
				return
			}
			resp := run(t, c, "rule", nil, tt.args...)
			taskWantText(t, "rule", tt.args, text(resp), tt.wantText, tt.wantAbsent)
			if got, ok := data(resp).(sdk.ColumnRule); !ok || got != tt.wantRule {
				t.Errorf("rule %v Data = %+v, want %+v", tt.args, data(resp), tt.wantRule)
			}

			column := tt.args[1]
			if got := ruleShow(t, c)[column]; got != tt.wantRule {
				t.Errorf("Rules.Show()[%s] = %+v, want %+v", column, got, tt.wantRule)
			}
			// The rule is persisted to the YAML file the guide names.
			raw, err := os.ReadFile(filepath.Join(h.Dir(), ".llmd", "rules", "default.yaml"))
			if err != nil || !strings.Contains(string(raw), column+":") {
				t.Errorf("default.yaml = %q, %v, want a %s entry", raw, err, column)
			}
			// Other columns are untouched.
			for col, want := range before {
				if col == column {
					continue
				}
				if got := ruleShow(t, c)[col]; got != want {
					t.Errorf("Rules.Show()[%s] = %+v, want unchanged %+v", col, got, want)
				}
			}
		})
	}
}

// TestRuleUnset covers removing automation: agent and role are cleared
// while the transitions stay, a manual column is a no-op, and a column
// with no rule is ErrNotFound.
func TestRuleUnset(t *testing.T) {
	tests := []struct {
		name     string
		noAuthor bool
		setup    func(t *testing.T, c sdk.Context)
		args     []string
		wantText []string
		wantRule sdk.ColumnRule
		wantErr  error
	}{
		{
			name: "clears agent and role, keeps transitions",
			setup: func(t *testing.T, c sdk.Context) {
				run(t, c, "rule", nil, "set", "code", "--agent", "claude-code", "--role", "developer", "--success", "test", "--failure", "blocked")
			},
			args:     []string{"unset", "code"},
			wantText: []string{"code", "[manual]", "test", "blocked"},
			wantRule: sdk.ColumnRule{Success: "test", Failure: "blocked"},
		},
		{
			name:     "manual default column is unchanged",
			args:     []string{"unset", "review"},
			wantText: []string{"review", "[manual]", "approval", "blocked"},
			wantRule: sdk.ColumnRule{Success: "approval", Failure: "blocked"},
		},
		{
			name: "resume survives unset",
			setup: func(t *testing.T, c sdk.Context) {
				run(t, c, "rule", nil, "set", "code", "--agent", "claude-code", "--resume")
			},
			args:     []string{"unset", "code"},
			wantText: []string{"[manual]"},
			wantRule: sdk.ColumnRule{Resume: true},
		},
		{
			name:     "works without an author",
			noAuthor: true,
			args:     []string{"unset", "review"},
			wantText: []string{"review"},
			wantRule: sdk.ColumnRule{Success: "approval", Failure: "blocked"},
		},
		{name: "column without a rule", args: []string{"unset", "backlog"}, wantErr: sdk.ErrNotFound},
		{name: "no column", args: []string{"unset"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			author := "alice"
			if tt.noAuthor {
				author = ""
			}
			_, c := ruleCLI(t, author)
			if tt.setup != nil {
				tt.setup(t, c)
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "rule", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("rule %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "rule", nil, tt.args...)
			taskWantText(t, "rule", tt.args, text(resp), tt.wantText, []string{"[claude-code"})
			if got, ok := data(resp).(sdk.ColumnRule); !ok || got != tt.wantRule {
				t.Errorf("rule %v Data = %+v, want %+v", tt.args, data(resp), tt.wantRule)
			}
			if got := ruleShow(t, c)[tt.args[1]]; got != tt.wantRule {
				t.Errorf("Rules.Show()[%s] = %+v, want %+v", tt.args[1], got, tt.wantRule)
			}
		})
	}
}
