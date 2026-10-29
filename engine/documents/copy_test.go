package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
)

func TestCopy(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *engine.Store)
		src     string
		dst     string
		want    string // content of both src and dst after the copy
		wantErr error
	}{
		{
			name:  "copies content to new path",
			setup: func(t *testing.T, s *engine.Store) { mustWrite(t, s, "docs/original", "content") },
			src:   "docs/original",
			dst:   "docs/copy",
			want:  "content",
		},
		{
			name:    "not found",
			src:     "nonexistent",
			dst:     "docs/copy",
			wantErr: documents.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, s)
			}

			doc, err := s.Documents.Copy(ctx, tt.src, tt.dst, testCopyOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Copy(%q, %q) error = %v, want %v", tt.src, tt.dst, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if doc.Content != tt.want {
				t.Errorf("Copy(%q, %q).Content = %q, want %q", tt.src, tt.dst, doc.Content, tt.want)
			}
			if doc.Path != tt.dst {
				t.Errorf("Copy(%q, %q).Path = %q, want %q", tt.src, tt.dst, doc.Path, tt.dst)
			}
			if doc.Version != 1 {
				t.Errorf("Copy(%q, %q).Version = %d, want 1", tt.src, tt.dst, doc.Version)
			}

			// The source must be left intact.
			orig, err := s.Documents.Read(ctx, tt.src)
			if err != nil {
				t.Fatalf("Read(%q) after Copy error = %v", tt.src, err)
			}
			if orig.Content != tt.want {
				t.Errorf("Read(%q) after Copy: Content = %q, want %q", tt.src, orig.Content, tt.want)
			}
		})
	}
}
