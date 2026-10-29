package history_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine/history"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func TestRevert(t *testing.T) {
	tests := []struct {
		name        string
		contents    []string // successive versions of docs/readme
		path        string   // document to revert; defaults to docs/readme
		version     int      // version to revert to
		opts        history.RevertOptions
		want        string // content after the revert
		wantVersion int
		wantMessage string
		wantErr     error
	}{
		{
			name:        "creates new version with old content",
			contents:    []string{"original content", "modified content"},
			version:     1,
			opts:        testRevertOpts(),
			want:        "original content",
			wantVersion: 3,
			wantMessage: "Reverted to version 1",
		},
		{
			name:        "custom message",
			contents:    []string{"v1", "v2"},
			version:     1,
			opts:        history.RevertOptions{Author: "test", Source: "cli", Message: "Rolling back bad change"},
			want:        "v1",
			wantVersion: 3,
			wantMessage: "Rolling back bad change",
		},
		{
			// Reverting to content identical to the latest version
			// returns the latest version instead of creating a new one.
			name:        "skips unchanged",
			contents:    []string{"same content", "different", "same content"},
			version:     1,
			opts:        testRevertOpts(),
			want:        "same content",
			wantVersion: 3,
		},
		{
			name:     "requires author",
			contents: []string{"content"},
			version:  1,
			opts:     history.RevertOptions{Source: "cli"},
			wantErr:  core.ErrAuthorRequired,
		},
		{
			name:    "missing document",
			path:    "docs/missing",
			version: 1,
			opts:    testRevertOpts(),
			wantErr: history.ErrNotFound,
		},
		{
			name:     "missing version",
			contents: []string{"only one"},
			version:  5,
			opts:     testRevertOpts(),
			wantErr:  history.ErrVersionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			for _, content := range tt.contents {
				mustWrite(t, s, "docs/readme", content)
			}

			path := "docs/readme"
			if tt.path != "" {
				path = tt.path
			}
			doc, err := s.History.Revert(ctx, path, tt.version, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Revert(%q, %d) error = %v, want %v", path, tt.version, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if doc.Content != tt.want {
				t.Errorf("Revert(%q, %d).Content = %q, want %q", "docs/readme", tt.version, doc.Content, tt.want)
			}
			if doc.Version != tt.wantVersion {
				t.Errorf("Revert(%q, %d).Version = %d, want %d", "docs/readme", tt.version, doc.Version, tt.wantVersion)
			}
			if doc.Message != tt.wantMessage {
				t.Errorf("Revert(%q, %d).Message = %q, want %q", "docs/readme", tt.version, doc.Message, tt.wantMessage)
			}

			// The reverted content must be what a plain read now returns.
			latest, err := s.Documents.Read(ctx, "docs/readme")
			if err != nil {
				t.Fatalf("Read() after Revert error = %v", err)
			}
			if latest.Content != tt.want {
				t.Errorf("Read() after Revert: Content = %q, want %q", latest.Content, tt.want)
			}
		})
	}
}
