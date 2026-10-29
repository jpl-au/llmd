package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestRestore covers recovery of a soft-deleted document: content,
// history and listing come back, and the error cases. Each case runs
// against a fresh store holding "docs/a" (two versions, then deleted)
// and "docs/live" (never deleted).
func TestRestore(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		author   string // "-" dispatches with an empty author
		wantText string
		wantErr  error
		skip     string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:     "restores the deleted document",
			args:     []string{"docs/a"},
			wantText: "Restored docs/a",
		},
		{
			name:    "document that was never deleted",
			args:    []string{"docs/live"},
			wantErr: sdk.ErrExists,
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no path",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# A v1")
			writeDoc(t, c, "docs/a", "# A v2")
			writeDoc(t, c, "docs/live", "# Live")
			run(t, c, "rm", nil, "docs/a")

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "restore", nil, tt.args...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("restore %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("restore %v: %v", tt.args, err)
			}
			if got := text(resp); got != tt.wantText {
				t.Errorf("restore %v = %q, want %q", tt.args, got, tt.wantText)
			}

			if got := docRead(t, c, "docs/a"); got != "# A v2" {
				t.Errorf("cat docs/a after restore = %q, want %q", got, "# A v2")
			}
			if versions := docVersions(t, c, "docs/a"); len(versions) != 2 {
				t.Errorf("history docs/a after restore has %d versions, want 2", len(versions))
			}
			if got := text(run(t, c, "ls", nil)); !strings.Contains(got, "docs/a") {
				t.Errorf("ls after restore = %q, want to contain docs/a", got)
			}
			if got := text(run(t, c, "ls", nil, "-la")); strings.Contains(got, "(deleted)") {
				t.Errorf("ls -la after restore = %q, must not flag anything deleted", got)
			}
			// Restoring twice fails: the document is live again.
			if err := runErr(t, c, "restore", nil, "docs/a"); !errors.Is(err, sdk.ErrExists) {
				t.Errorf("second restore error = %v, want %v", err, sdk.ErrExists)
			}
		})
	}
}
