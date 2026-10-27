package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestKeyToPath(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the store and returns the key to resolve.
		setup   func(t *testing.T, s *llmd.Store) string
		want    string
		wantErr error
	}{
		{
			name: "resolves key to path",
			setup: func(t *testing.T, s *llmd.Store) string {
				return mustWrite(t, s, "docs/readme", "hello world").Key
			},
			want: "docs/readme",
		},
		{
			name: "stable across versions",
			setup: func(t *testing.T, s *llmd.Store) string {
				v1 := mustWrite(t, s, "docs/readme", "v1 content")
				v2 := mustWrite(t, s, "docs/readme", "v2 content")
				if v1.Key != v2.Key {
					t.Fatalf("Key changed between versions: v1=%q v2=%q", v1.Key, v2.Key)
				}
				return v1.Key
			},
			want: "docs/readme",
		},
		{
			name:    "unknown key",
			setup:   func(t *testing.T, s *llmd.Store) string { return "zzzzzzzzzz" },
			wantErr: documents.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			key := tt.setup(t, s)

			got, err := s.Documents.KeyToPath(t.Context(), key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("KeyToPath(%q) error = %v, want %v", key, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("KeyToPath(%q) = %q, want %q", key, got, tt.want)
			}
		})
	}
}
