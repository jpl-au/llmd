package host

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/search"
	docpath "github.com/jpl-au/llmd/internal/path"
	"github.com/jpl-au/llmd/sdk"
)

// TestDocumentsPreview covers the title and leading blank lines being
// skipped, the line cap, and the empty results for a missing path or
// a zero count.
func TestDocumentsPreview(t *testing.T) {
	tests := []struct {
		name    string
		content string
		write   bool
		path    string
		n       int
		want    string
		wantErr error
	}{
		{"skips the title", "# Title\n\nfirst\nsecond\nthird\n", true, "doc", 2, "first\nsecond", nil},
		{"skips leading blank lines", "\n\n  \nfirst\nsecond", true, "doc", 5, "first\nsecond", nil},
		{"keeps blank lines after the start", "# T\nfirst\n\nthird", true, "doc", 3, "first\n\nthird", nil},
		{"no title", "first\nsecond", true, "doc", 1, "first", nil},
		{"heading-like line later is kept", "# T\nfirst\n# Section\n", true, "doc", 2, "first\n# Section", nil},
		{"only a title", "# Title\n", true, "doc", 3, "", nil},
		{"cap larger than the document", "# T\nonly", true, "doc", 10, "only", nil},
		{"zero lines", "# T\nfirst", true, "doc", 0, "", nil},
		{"negative lines", "# T\nfirst", true, "doc", -1, "", nil},
		{"empty path", "# T\nfirst", true, "", 3, "", nil},
		{"missing document", "", false, "doc", 3, "", sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if tt.write {
				writeDoc(t, c, "doc", tt.content)
			}
			got, err := c.Documents.Preview(tt.path, tt.n)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Preview(%q, %d) error = %v, want %v", tt.path, tt.n, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Preview(%q, %d) = %q, want %q", tt.path, tt.n, got, tt.want)
			}
		})
	}
}

// TestDocumentsMoveErrors covers the validation and conflict errors
// of Move; the missing-source case is in TestNotFound.
func TestDocumentsMoveErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, c sdk.Context)
		from    string
		to      string
		wantErr error
	}{
		{
			name:    "destination exists",
			setup:   func(t *testing.T, c sdk.Context) { writeDoc(t, c, "a", "a"); writeDoc(t, c, "b", "b") },
			from:    "a",
			to:      "b",
			wantErr: sdk.ErrExists,
		},
		{
			name:    "invalid destination",
			setup:   func(t *testing.T, c sdk.Context) { writeDoc(t, c, "a", "a") },
			from:    "a",
			to:      "../escape",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "invalid source",
			from:    "../escape",
			to:      "b",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "null byte in source",
			setup:   func(t *testing.T, c sdk.Context) { writeDoc(t, c, "a", "a") },
			from:    "bad\x00",
			to:      "b",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "null byte in destination",
			setup:   func(t *testing.T, c sdk.Context) { writeDoc(t, c, "a", "a") },
			from:    "a",
			to:      "bad\x00",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "both invalid",
			from:    "bad\x00",
			to:      "../escape",
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name: "deleted source",
			setup: func(t *testing.T, c sdk.Context) {
				writeDoc(t, c, "a", "a")
				if err := c.Documents.Delete("a", sdk.DeleteOpts{Author: "alice"}); err != nil {
					t.Fatal(err)
				}
			},
			from:    "a",
			to:      "b",
			wantErr: sdk.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			err := c.Documents.Move(tt.from, tt.to, sdk.MoveOpts{Author: "alice"})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Move(%q, %q) error = %v, want %v", tt.from, tt.to, err, tt.wantErr)
			}
			if tt.setup != nil && tt.from == "a" && !errors.Is(tt.wantErr, sdk.ErrNotFound) {
				if ok, _ := c.Documents.Exists("a"); !ok {
					t.Errorf("source %q gone after a failed Move", tt.from)
				}
			}
		})
	}
}

// TestDocErr covers the translation of every document, path, search
// and history sentinel.
func TestDocErr(t *testing.T) {
	other := errors.New("disk on fire")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"document not found", documents.ErrNotFound, sdk.ErrNotFound},
		{"document deleted", documents.ErrDeleted, sdk.ErrNotFound},
		{"destination exists", documents.ErrExists, sdk.ErrExists},
		{"invalid path", docpath.ErrInvalid, sdk.ErrInvalidArg},
		{"path too long", docpath.ErrTooLong, sdk.ErrInvalidArg},
		{"invalid glob", search.ErrInvalidGlob, sdk.ErrInvalidArg},
		{"invalid query", search.ErrInvalidQuery, sdk.ErrInvalidArg},
		{"history not found", history.ErrNotFound, sdk.ErrNotFound},
		{"invalid version", history.ErrVersionInvalid, sdk.ErrInvalidArg},
		{"no match", documents.ErrNoMatch, sdk.ErrNoMatch},
		{"not unique", documents.ErrNotUnique, sdk.ErrNotUnique},
		{"no-op", documents.ErrNoOp, sdk.ErrNoOp},
		{"passthrough", other, other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := docErr(tt.in)
			if !errors.Is(got, tt.want) {
				t.Errorf("docErr(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.in != nil && !errors.Is(got, tt.in) {
				t.Errorf("docErr(%v) = %v, want to wrap the original", tt.in, got)
			}
		})
	}
}

// TestDocumentsSentinelsReachSDK checks a few internal failures
// through the bridge so the mapping is exercised end to end, not only
// in docErr.
func TestDocumentsSentinelsReachSDK(t *testing.T) {
	tests := []struct {
		name    string
		call    func(t *testing.T, c sdk.Context) error
		wantErr error
	}{
		{"read a deleted document", func(t *testing.T, c sdk.Context) error {
			writeDoc(t, c, "gone", "x")
			if err := c.Documents.Delete("gone", sdk.DeleteOpts{Author: "alice"}); err != nil {
				return err
			}
			_, err := c.Documents.Read("gone", sdk.ReadOpts{})
			return err
		}, sdk.ErrNotFound},
		{"revert to a version that does not exist", func(t *testing.T, c sdk.Context) error {
			writeDoc(t, c, "doc", "x")
			return c.Documents.Revert("doc", 9, sdk.RevertOpts{Author: "alice"})
		}, sdk.ErrInvalidArg},
		{"grep with a bad query", func(t *testing.T, c sdk.Context) error {
			writeDoc(t, c, "doc", "x")
			_, err := c.Documents.Grep("foo AND ", sdk.GrepOpts{})
			return err
		}, sdk.ErrInvalidArg},
		{"export a missing document", func(t *testing.T, c sdk.Context) error {
			_, err := c.Documents.Export("missing", t.TempDir(), sdk.ExportOpts{})
			return err
		}, sdk.ErrNotFound},
		{"glob with a bad pattern", func(_ *testing.T, c sdk.Context) error {
			_, err := c.Documents.Glob("[")
			return err
		}, sdk.ErrInvalidArg},
		{"read a path that escapes", func(_ *testing.T, c sdk.Context) error {
			_, err := c.Documents.Read("../escape", sdk.ReadOpts{})
			return err
		}, sdk.ErrInvalidArg},
		{"list a prefix that escapes", func(_ *testing.T, c sdk.Context) error {
			_, err := c.Documents.List("../", sdk.ListOpts{})
			return err
		}, sdk.ErrInvalidArg},
		{"grep a path that escapes", func(_ *testing.T, c sdk.Context) error {
			_, err := c.Documents.Grep("x", sdk.GrepOpts{Path: "../"})
			return err
		}, sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if err := tt.call(t, c); !errors.Is(err, tt.wantErr) {
				t.Errorf("%s error = %v, want %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
