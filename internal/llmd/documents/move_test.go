package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestMove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *llmd.Store)
		src     string
		dst     string
		want    string // content readable at dst after the move
		wantErr error
	}{
		{
			name:  "moves content to new path",
			setup: func(t *testing.T, s *llmd.Store) { mustWrite(t, s, "docs/old", "content") },
			src:   "docs/old",
			dst:   "docs/new",
			want:  "content",
		},
		{
			name:    "not found",
			src:     "nonexistent",
			dst:     "docs/new",
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

			err := s.Documents.Move(ctx, tt.src, tt.dst, testMoveOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Move(%q, %q) error = %v, want %v", tt.src, tt.dst, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if _, err := s.Documents.Read(ctx, tt.src); !errors.Is(err, documents.ErrNotFound) {
				t.Errorf("Read(%q) after Move error = %v, want %v", tt.src, err, documents.ErrNotFound)
			}
			doc, err := s.Documents.Read(ctx, tt.dst)
			if err != nil {
				t.Fatalf("Read(%q) after Move error = %v", tt.dst, err)
			}
			if doc.Content != tt.want {
				t.Errorf("Read(%q) after Move: Content = %q, want %q", tt.dst, doc.Content, tt.want)
			}
		})
	}
}
