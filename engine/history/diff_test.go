package history_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the store and returns the two identifiers to diff.
		setup   func(t *testing.T, s *engine.Store) (a, b string)
		wantA   string
		wantB   string
		wantErr error
	}{
		{
			// Key is stable across versions, so key:version addresses
			// each version of one document.
			name: "key with version",
			setup: func(t *testing.T, s *engine.Store) (string, string) {
				doc := mustWrite(t, s, "docs/readme", "first version")
				mustWrite(t, s, "docs/readme", "second version")
				return doc.Key + ":1", doc.Key + ":2"
			},
			wantA: "first version",
			wantB: "second version",
		},
		{
			name: "by path",
			setup: func(t *testing.T, s *engine.Store) (string, string) {
				mustWrite(t, s, "docs/readme", "readme content")
				mustWrite(t, s, "docs/api", "api content")
				return "docs/readme", "docs/api"
			},
			wantA: "readme content",
			wantB: "api content",
		},
		{
			name: "not found",
			setup: func(t *testing.T, s *engine.Store) (string, string) {
				mustWrite(t, s, "docs/readme", "content")
				return "docs/readme", "docs/nonexistent"
			},
			wantErr: documents.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			a, b := tt.setup(t, s)

			result, err := s.History.Diff(t.Context(), a, b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Diff(%q, %q) error = %v, want %v", a, b, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if result.A.Content != tt.wantA {
				t.Errorf("Diff(%q, %q).A.Content = %q, want %q", a, b, result.A.Content, tt.wantA)
			}
			if result.B.Content != tt.wantB {
				t.Errorf("Diff(%q, %q).B.Content = %q, want %q", a, b, result.B.Content, tt.wantB)
			}
		})
	}
}
