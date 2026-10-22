package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// Helpers shared by the document command tests. They carry a "doc"
// prefix so they cannot collide with helpers other test groups add.

// docRunAs dispatches cmd as the given author, which may be empty to
// exercise the host's NeedsAuthor check, and returns the response and
// error so a mutating command is dispatched exactly once per case.
func docRunAs(t *testing.T, c sdk.Context, author, cmd string, stdin []byte, args ...string) (sdk.Response, error) {
	t.Helper()
	return c.Runtime.Dispatch(t.Context(), cmd, args, author, stdin, "")
}

// docKey returns the 9-character key of the document at path.
func docKey(t *testing.T, c sdk.Context, path string) string {
	t.Helper()
	docs, err := c.Documents.List(path, sdk.ListOpts{Deleted: true})
	if err != nil {
		t.Fatalf("List(%s) = %v", path, err)
	}
	for _, d := range docs {
		if d.Path == path {
			return d.Key
		}
	}
	t.Fatalf("List(%s) did not return the document", path)
	return ""
}

// docRead returns the current content of path through the cat command.
func docRead(t *testing.T, c sdk.Context, path string) string {
	t.Helper()
	return text(run(t, c, "cat", nil, path))
}

// docVersions returns the full version history of path, newest first.
func docVersions(t *testing.T, c sdk.Context, path string) []sdk.Version {
	t.Helper()
	versions, err := c.Documents.History(path, 0)
	if err != nil {
		t.Fatalf("History(%s) = %v", path, err)
	}
	return versions
}

// docArgs substitutes {key} in args with the key of the document at
// path so tests can address documents by key without knowing it ahead
// of time.
func docArgs(t *testing.T, c sdk.Context, path string, args []string) []string {
	t.Helper()
	out := make([]string, len(args))
	var key string
	for i, a := range args {
		if strings.Contains(a, "{key}") {
			if key == "" {
				key = docKey(t, c, path)
			}
			a = strings.ReplaceAll(a, "{key}", key)
		}
		out[i] = a
	}
	return out
}

// TestWrite covers document creation, versioning on update, the
// no-op when content is unchanged, the --message flag, line-ending
// normalisation, key addressing, and the error cases. Each case runs
// against a fresh store holding "docs/a" at version 1.
func TestWrite(t *testing.T) {
	tests := []struct {
		name         string
		args         []string // {key} is replaced with the key of docs/a
		stdin        string
		author       string // defaults to alice
		wantText     string // exact response text
		wantContent  string // content of wantPath after the write
		wantPath     string // defaults to docs/a
		wantVersions int    // history length of wantPath (0 = don't check)
		wantMessage  string // message of the newest version
		wantErr      error
		wantAnyErr   bool // any error is acceptable (no sentinel is defined)
	}{
		{
			name:         "creates a new document at version 1",
			args:         []string{"docs/new"},
			stdin:        "# New",
			wantText:     "Wrote docs/new",
			wantPath:     "docs/new",
			wantContent:  "# New",
			wantVersions: 1,
		},
		{
			name:         "updating an existing document adds a version",
			args:         []string{"docs/a"},
			stdin:        "# Second",
			wantText:     "Wrote docs/a",
			wantContent:  "# Second",
			wantVersions: 2,
		},
		{
			name:         "unchanged content is a no-op",
			args:         []string{"docs/a"},
			stdin:        "# First",
			wantText:     "Wrote docs/a",
			wantContent:  "# First",
			wantVersions: 1,
		},
		{
			name:         "message is recorded",
			args:         []string{"--message", "revised", "docs/a"},
			stdin:        "# Second",
			wantText:     "Wrote docs/a",
			wantContent:  "# Second",
			wantVersions: 2,
			wantMessage:  "revised",
		},
		{
			name:         "message with equals form",
			args:         []string{"docs/a", "--message=revised"},
			stdin:        "# Second",
			wantText:     "Wrote docs/a",
			wantContent:  "# Second",
			wantVersions: 2,
			wantMessage:  "revised",
		},
		{
			name:        "CRLF line endings are normalised",
			args:        []string{"docs/crlf"},
			stdin:       "one\r\ntwo\rthree",
			wantText:    "Wrote docs/crlf",
			wantPath:    "docs/crlf",
			wantContent: "one\ntwo\nthree",
		},
		{
			name:         "empty stdin writes an empty document",
			args:         []string{"docs/empty"},
			stdin:        "",
			wantText:     "Wrote docs/empty",
			wantPath:     "docs/empty",
			wantContent:  "",
			wantVersions: 1,
		},
		{
			name:         "by key updates the existing document",
			args:         []string{"{key}"},
			stdin:        "# Via key",
			wantContent:  "# Via key",
			wantVersions: 2,
		},
		{
			name:    "no path",
			args:    nil,
			stdin:   "x",
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a"},
			stdin:   "x",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "message without a value",
			args:    []string{"docs/a", "--message"},
			stdin:   "x",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "null byte in content",
			args:    []string{"docs/a"},
			stdin:   "a\x00b",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:       "path traversal is rejected",
			args:       []string{"../escape"},
			stdin:      "x",
			wantAnyErr: true,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a"},
			stdin:   "x",
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# First")
			args := docArgs(t, c, "docs/a", tt.args)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "write", []byte(tt.stdin), args...)
			if tt.wantAnyErr {
				if err == nil {
					t.Errorf("write %v error = nil, want an error", args)
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("write %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("write %v: %v", args, err)
			}

			if tt.wantText != "" && text(resp) != tt.wantText {
				t.Errorf("write %v = %q, want %q", args, text(resp), tt.wantText)
			}
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("write %v response type = %T, want sdk.Text", args, resp)
			}

			path := tt.wantPath
			if path == "" {
				path = "docs/a"
			}
			if got := docRead(t, c, path); got != tt.wantContent {
				t.Errorf("cat %s after write = %q, want %q", path, got, tt.wantContent)
			}
			versions := docVersions(t, c, path)
			if tt.wantVersions != 0 && len(versions) != tt.wantVersions {
				t.Errorf("history %s has %d versions, want %d", path, len(versions), tt.wantVersions)
			}
			if tt.wantMessage != "" && versions[0].Message != tt.wantMessage {
				t.Errorf("newest version message = %q, want %q", versions[0].Message, tt.wantMessage)
			}
		})
	}
}
