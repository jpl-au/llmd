package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
)

func TestDelete(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *engine.Store)
		path    string
		wantErr error
	}{
		{
			name:  "soft-deletes existing document",
			setup: func(t *testing.T, s *engine.Store) { mustWrite(t, s, "docs/readme", "content") },
			path:  "docs/readme",
		},
		{
			name:    "not found",
			path:    "nonexistent",
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

			err := s.Documents.Delete(ctx, tt.path, testDeleteOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Delete(%q) error = %v, want %v", tt.path, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			// A deleted document is hidden from List and reads as ErrDeleted.
			list, err := s.Documents.List(ctx)
			if err != nil {
				t.Fatalf("List() after Delete error = %v", err)
			}
			if len(list) != 0 {
				t.Errorf("List() after Delete(%q) returned %d items, want 0", tt.path, len(list))
			}
			if _, err := s.Documents.Read(ctx, tt.path); !errors.Is(err, documents.ErrDeleted) {
				t.Errorf("Read(%q) after Delete error = %v, want %v", tt.path, err, documents.ErrDeleted)
			}
		})
	}
}
