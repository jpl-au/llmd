package documents_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		content   string
		opts      documents.WriteOptions
		wantSize  int
		wantLines int
		wantErr   error
	}{
		{
			name:      "creates first version",
			path:      "docs/readme",
			content:   "# Hello",
			opts:      testWriteOpts(),
			wantSize:  7,
			wantLines: 1,
		},
		{
			name:    "requires author",
			path:    "docs/readme",
			content: "content",
			opts:    documents.WriteOptions{Source: "cli"},
			wantErr: core.ErrAuthorRequired,
		},
		{
			name:    "requires source",
			path:    "docs/readme",
			content: "content",
			opts:    documents.WriteOptions{Author: "test"},
			wantErr: core.ErrSourceRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)

			doc, err := s.Documents.Write(t.Context(), tt.path, tt.content, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Write(%q) error = %v, want %v", tt.path, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if doc.Path != tt.path {
				t.Errorf("Write(%q).Path = %q, want %q", tt.path, doc.Path, tt.path)
			}
			if doc.Content != tt.content {
				t.Errorf("Write(%q).Content = %q, want %q", tt.path, doc.Content, tt.content)
			}
			if doc.Version != 1 {
				t.Errorf("Write(%q).Version = %d, want 1", tt.path, doc.Version)
			}
			if doc.Author != tt.opts.Author {
				t.Errorf("Write(%q).Author = %q, want %q", tt.path, doc.Author, tt.opts.Author)
			}
			if len(doc.Key) != 10 {
				t.Errorf("Write(%q).Key = %q, want 10 characters", tt.path, doc.Key)
			}
			if doc.Meta == nil {
				t.Errorf("Write(%q).Meta = nil, want non-nil", tt.path)
			} else {
				if doc.Meta.Size != tt.wantSize {
					t.Errorf("Write(%q).Meta.Size = %d, want %d", tt.path, doc.Meta.Size, tt.wantSize)
				}
				if doc.Meta.Lines != tt.wantLines {
					t.Errorf("Write(%q).Meta.Lines = %d, want %d", tt.path, doc.Meta.Lines, tt.wantLines)
				}
			}
		})
	}
}

// TestWrite_Versioning writes successive contents to one path and checks
// the version each write reports. The key must stay stable throughout.
func TestWrite_Versioning(t *testing.T) {
	tests := []struct {
		name         string
		contents     []string // successive writes to docs/readme
		wantVersions []int
	}{
		{
			name:         "new version per change",
			contents:     []string{"v1", "v2", "v3"},
			wantVersions: []int{1, 2, 3},
		},
		{
			name:         "skips unchanged content",
			contents:     []string{"same content", "same content"},
			wantVersions: []int{1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)

			versions := make([]int, len(tt.contents))
			keys := make([]string, len(tt.contents))
			for i, content := range tt.contents {
				doc := mustWrite(t, s, "docs/readme", content)
				versions[i] = doc.Version
				keys[i] = doc.Key
			}

			if !slices.Equal(versions, tt.wantVersions) {
				t.Errorf("Write(%q) versions = %v, want %v", tt.contents, versions, tt.wantVersions)
			}
			for i, k := range keys {
				if k != keys[0] {
					t.Errorf("Write(%q) key = %q, want %q (stable across versions)", tt.contents[i], k, keys[0])
				}
			}
		})
	}
}
