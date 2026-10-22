package host

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// defaultRules is the rule set seeded on first access: manual
// transitions for the default board's working columns.
var defaultRules = map[string]sdk.ColumnRule{
	"up-next":     {Success: "in-progress", Failure: "up-next"},
	"in-progress": {Success: "review", Failure: "blocked"},
	"review":      {Success: "approval", Failure: "blocked"},
}

// corruptRules overwrites the default rule file with invalid YAML.
func corruptRules(t *testing.T, h *Host) {
	t.Helper()
	path := filepath.Join(h.Dir(), ".llmd", "rules", "default.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("review: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRulesShow(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, h *Host, c sdk.Context)
		want    map[string]sdk.ColumnRule
		wantErr bool
	}{
		{name: "defaults are seeded", want: defaultRules},
		{
			name: "after set",
			setup: func(t *testing.T, _ *Host, c sdk.Context) {
				setRule(t, c, "review", sdk.ColumnRule{Agent: "claude-code", Role: "auditor", Success: "approval", Failure: "in-progress", Resume: true})
			},
			want: map[string]sdk.ColumnRule{
				"up-next":     {Success: "in-progress", Failure: "up-next"},
				"in-progress": {Success: "review", Failure: "blocked"},
				"review":      {Agent: "claude-code", Role: "auditor", Success: "approval", Failure: "in-progress", Resume: true},
			},
		},
		{name: "corrupt file", setup: func(t *testing.T, h *Host, _ sdk.Context) { corruptRules(t, h) }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := TestSetup(t, TestIsolated)
			c := h.Context(t.Context(), "alice")
			if tt.setup != nil {
				tt.setup(t, h, c)
			}
			got, err := c.Rules.Show()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Show() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Show() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRulesSet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, h *Host, c sdk.Context)
		column  string
		rule    sdk.ColumnRule
		wantErr bool
	}{
		{name: "new column", column: "qa", rule: sdk.ColumnRule{Agent: "gemini", Success: "done", Failure: "in-progress"}},
		{name: "replaces an existing rule", column: "in-progress", rule: sdk.ColumnRule{Agent: "claude-code", Role: "developer", Success: "review", Failure: "blocked"}},
		{name: "manual rule", column: "approval", rule: sdk.ColumnRule{Success: "done", Failure: "review"}},
		{name: "corrupt file", setup: func(t *testing.T, h *Host, _ sdk.Context) { corruptRules(t, h) }, column: "qa", rule: sdk.ColumnRule{Success: "done"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := TestSetup(t, TestIsolated)
			c := h.Context(t.Context(), "alice")
			if tt.setup != nil {
				tt.setup(t, h, c)
			}
			err := c.Rules.Set(tt.column, tt.rule)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Set(%q, %+v) error = %v, wantErr %v", tt.column, tt.rule, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			rs, err := c.Rules.Show()
			if err != nil {
				t.Fatalf("Show() error = %v", err)
			}
			if got := rs[tt.column]; got != tt.rule {
				t.Errorf("Show()[%q] after Set = %+v, want %+v", tt.column, got, tt.rule)
			}
			// Other columns keep their defaults.
			for col, want := range defaultRules {
				if col == tt.column {
					continue
				}
				if rs[col] != want {
					t.Errorf("Show()[%q] after Set(%q) = %+v, want %+v unchanged", col, tt.column, rs[col], want)
				}
			}
		})
	}
}

func TestRulesUnset(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, h *Host, c sdk.Context)
		column  string
		want    sdk.ColumnRule
		wantErr error
		wantAny bool
	}{
		{
			name: "clears the agent and keeps the transitions",
			setup: func(t *testing.T, _ *Host, c sdk.Context) {
				setRule(t, c, "review", sdk.ColumnRule{Agent: "claude-code", Role: "auditor", Success: "approval", Failure: "blocked", Resume: true})
			},
			column: "review",
			want:   sdk.ColumnRule{Success: "approval", Failure: "blocked", Resume: true},
		},
		{
			name:   "manual column is unchanged",
			column: "in-progress",
			want:   sdk.ColumnRule{Success: "review", Failure: "blocked"},
		},
		{name: "missing column", column: "nowhere", wantErr: sdk.ErrNotFound},
		{name: "corrupt file", setup: func(t *testing.T, h *Host, _ sdk.Context) { corruptRules(t, h) }, column: "review", wantAny: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := TestSetup(t, TestIsolated)
			c := h.Context(t.Context(), "alice")
			if tt.setup != nil {
				tt.setup(t, h, c)
			}
			err := c.Rules.Unset(tt.column)
			switch {
			case tt.wantAny:
				if err == nil {
					t.Fatalf("Unset(%q) error = nil, want an error", tt.column)
				}
				return
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Unset(%q) error = %v, want %v", tt.column, err, tt.wantErr)
				}
				return
			case err != nil:
				t.Fatalf("Unset(%q) error = %v", tt.column, err)
			}
			rs, err := c.Rules.Show()
			if err != nil {
				t.Fatalf("Show() error = %v", err)
			}
			if got := rs[tt.column]; got != tt.want {
				t.Errorf("Show()[%q] after Unset = %+v, want %+v", tt.column, got, tt.want)
			}
		})
	}
}
