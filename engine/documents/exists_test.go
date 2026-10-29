package documents_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine"
)

func TestExists(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the store and returns the path or key to look up.
		setup func(t *testing.T, s *engine.Store) string
		want  bool
	}{
		{
			name:  "missing document",
			setup: func(t *testing.T, s *engine.Store) string { return "docs/readme" },
			want:  false,
		},
		{
			name: "by path",
			setup: func(t *testing.T, s *engine.Store) string {
				mustWrite(t, s, "docs/readme", "# Hello")
				return "docs/readme"
			},
			want: true,
		},
		{
			name: "by key",
			setup: func(t *testing.T, s *engine.Store) string {
				return mustWrite(t, s, "docs/readme", "# Hello").Key
			},
			want: true,
		},
		{
			name: "deleted document",
			setup: func(t *testing.T, s *engine.Store) string {
				mustWrite(t, s, "docs/readme", "# Hello")
				mustDelete(t, s, "docs/readme")
				return "docs/readme"
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			value := tt.setup(t, s)

			got, err := s.Documents.Exists(t.Context(), value)
			if err != nil {
				t.Fatalf("Exists(%q) error = %v", value, err)
			}
			if got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", value, got, tt.want)
			}
		})
	}
}
