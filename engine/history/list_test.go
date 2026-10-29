package history_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/engine/history"
)

func TestList(t *testing.T) {
	tests := []struct {
		name    string
		writes  int // versions of docs/readme to write before listing
		path    string
		opts    history.ListOptions
		want    []int // versions, newest first
		wantErr error
	}{
		{
			name:   "all versions newest first",
			writes: 3,
			path:   "docs/readme",
			want:   []int{3, 2, 1},
		},
		{
			name:   "limit",
			writes: 5,
			path:   "docs/readme",
			opts:   history.ListOptions{Limit: 2},
			want:   []int{5, 4},
		},
		{
			name:    "not found",
			path:    "nonexistent",
			wantErr: history.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			for i := 1; i <= tt.writes; i++ {
				mustWrite(t, s, "docs/readme", fmt.Sprintf("version %d", i))
			}

			versions, err := s.History.List(t.Context(), tt.path, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("List(%q, %+v) error = %v, want %v", tt.path, tt.opts, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			got := make([]int, len(versions))
			for i, v := range versions {
				got[i] = v.Version
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%q, %+v) versions = %v, want %v", tt.path, tt.opts, got, tt.want)
			}
		})
	}
}
