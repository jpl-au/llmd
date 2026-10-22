package cli

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// TestGuide covers the overview, command and workflow topics, the
// install alias, the unknown-topic error with its topic list, and
// flag parsing. guide is storeless, so each case dispatches through
// both a store-backed and a storeless host.
func TestGuide(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantText   []string
		wantAbsent []string
		wantErr    error
		wantErrMsg []string // substrings of the error message
	}{
		{
			name:     "overview",
			args:     nil,
			wantText: []string{"# llmd guide", "## Commands", "`cat`", "`vacuum`"},
		},
		{
			name:     "overview by name",
			args:     []string{"guide"},
			wantText: []string{"# llmd guide"},
		},
		{
			name:       "command topic",
			args:       []string{"cat"},
			wantText:   []string{"# llmd cat", "## Usage"},
			wantAbsent: []string{"# llmd guide"},
		},
		{
			name:     "workflow topic",
			args:     []string{"workflow"},
			wantText: []string{"# "},
		},
		{
			name:     "install resolves to the platform guide",
			args:     []string{"install"},
			wantText: []string{"# Installing llmd on " + platformName()},
		},
		{
			name:       "unknown topic",
			args:       []string{"nope"},
			wantErr:    sdk.ErrNotFound,
			wantErrMsg: []string{"nope", "Available:", "cat", "workflow"},
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := newCLI(t, "alice")
			storeless := host.New().Context(t.Context(), "")
			for label, c := range map[string]sdk.Context{"store": stored, "storeless": storeless} {
				if tt.wantErr != nil {
					err := runErr(t, c, "guide", nil, tt.args...)
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("[%s] guide %v error = %v, want %v", label, tt.args, err, tt.wantErr)
					}
					for _, want := range tt.wantErrMsg {
						if err == nil || !strings.Contains(err.Error(), want) {
							t.Errorf("[%s] guide %v error = %v, want to mention %q", label, tt.args, err, want)
						}
					}
					continue
				}
				resp := run(t, c, "guide", nil, tt.args...)
				md, ok := resp.(sdk.Markdown)
				if !ok {
					t.Errorf("[%s] guide %v response type = %T, want sdk.Markdown", label, tt.args, resp)
				}
				if md.Data != nil {
					t.Errorf("[%s] guide %v Data = %v, want nil (markdown is the payload)", label, tt.args, md.Data)
				}
				got := text(resp)
				for _, want := range tt.wantText {
					if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
						t.Errorf("[%s] guide %v = %.80q..., want to contain %q", label, tt.args, got, want)
					}
				}
				for _, absent := range tt.wantAbsent {
					if strings.Contains(got, absent) {
						t.Errorf("[%s] guide %v must not contain %q", label, tt.args, absent)
					}
				}
			}
		})
	}
}

// TestGuideCoversCommands checks that every registered command with
// a guide page is reachable by name and that the overview mentions
// each of the commands under test here.
func TestGuideCoversCommands(t *testing.T) {
	c := newCLI(t, "alice")
	overview := text(run(t, c, "guide", nil))
	for _, cmd := range []string{"tag", "link", "unlink", "import", "export", "mirror", "vacuum", "version", "config", "init", "guide", "status", "review"} {
		if !strings.Contains(overview, "`"+cmd+"`") {
			t.Errorf("guide overview does not mention `%s`", cmd)
		}
		if cmd == "version" {
			continue // version has no guide page
		}
		got := text(run(t, c, "guide", nil, cmd))
		if !strings.HasPrefix(got, "# llmd "+cmd) {
			t.Errorf("guide %s = %.40q, want to start with %q", cmd, got, "# llmd "+cmd)
		}
	}
}

// platformName is how the install guides name the current platform.
func platformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}
