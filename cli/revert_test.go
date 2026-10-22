package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestRevert covers rolling back to an earlier version: a new version
// is appended rather than history rewritten, the "v" prefix, the
// default and custom messages, key addressing, the no-op when the
// target matches the current content, and the error cases. Each case
// runs against a fresh store holding "docs/a" at versions 1 to 3.
func TestRevert(t *testing.T) {
	tests := []struct {
		name         string
		args         []string // {key} is replaced with the key of docs/a
		author       string   // "-" dispatches with an empty author
		wantText     string
		wantContent  string // content of docs/a afterwards
		wantVersions int    // history length afterwards
		wantMessage  string // message of the newest version
		wantErr      error
	}{
		{
			name:         "reverts to version 1",
			args:         []string{"docs/a", "1"},
			wantText:     "Reverted docs/a to version 1",
			wantContent:  "v1",
			wantVersions: 4,
			wantMessage:  "Reverted to version 1",
		},
		{
			name:         "v prefix is accepted",
			args:         []string{"docs/a", "v2"},
			wantText:     "Reverted docs/a to version 2",
			wantContent:  "v2",
			wantVersions: 4,
			wantMessage:  "Reverted to version 2",
		},
		{
			name:         "custom message",
			args:         []string{"docs/a", "1", "--message", "roll back"},
			wantText:     "Reverted docs/a to version 1",
			wantContent:  "v1",
			wantVersions: 4,
			wantMessage:  "roll back",
		},
		{
			name:         "by key",
			args:         []string{"{key}", "1"},
			wantText:     "to version 1",
			wantContent:  "v1",
			wantVersions: 4,
		},
		{
			name:         "reverting to the current content is a no-op",
			args:         []string{"docs/a", "3"},
			wantText:     "Reverted docs/a to version 3",
			wantContent:  "v3",
			wantVersions: 3,
		},
		{
			name:    "non-numeric version",
			args:    []string{"docs/a", "latest"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "version that does not exist",
			args:    []string{"docs/a", "9"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			// The version is positional; a --version flag is rejected
			// (test/smoke_cli.sh passes --version and ignores the exit
			// code, so its revert step silently does nothing).
			name:    "version flag is not accepted",
			args:    []string{"docs/a", "--version", "1"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing", "1"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no version",
			args:    []string{"docs/a"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a", "1"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a", "1"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "v1")
			writeDoc(t, c, "docs/a", "v2")
			writeDoc(t, c, "docs/a", "v3")
			args := docArgs(t, c, "docs/a", tt.args)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "revert", nil, args...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("revert %v error = %v, want %v", args, err, tt.wantErr)
				}
				if got := docRead(t, c, "docs/a"); got != "v3" {
					t.Errorf("cat docs/a after failed revert = %q, want %q", got, "v3")
				}
				return
			}
			if err != nil {
				t.Fatalf("revert %v: %v", args, err)
			}
			if got := text(resp); !strings.HasSuffix(got, tt.wantText) {
				t.Errorf("revert %v = %q, want suffix %q", args, got, tt.wantText)
			}

			if got := docRead(t, c, "docs/a"); got != tt.wantContent {
				t.Errorf("cat docs/a after revert = %q, want %q", got, tt.wantContent)
			}
			versions := docVersions(t, c, "docs/a")
			if len(versions) != tt.wantVersions {
				t.Errorf("history docs/a has %d versions, want %d", len(versions), tt.wantVersions)
			}
			if tt.wantMessage != "" && versions[0].Message != tt.wantMessage {
				t.Errorf("newest version message = %q, want %q", versions[0].Message, tt.wantMessage)
			}
			// Revert is non-destructive: version 1 still reads as v1.
			if got := text(run(t, c, "cat", nil, "--version", "1", "docs/a")); got != "v1" {
				t.Errorf("cat --version 1 after revert = %q, want %q", got, "v1")
			}
		})
	}
}
