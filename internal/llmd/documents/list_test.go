package documents_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestList(t *testing.T) {
	three := func(t *testing.T, s *llmd.Store) {
		mustWrite(t, s, "docs/readme", "readme")
		mustWrite(t, s, "docs/api", "api")
		mustWrite(t, s, "notes/todo", "todo")
	}
	versioned := func(t *testing.T, s *llmd.Store) {
		mustWrite(t, s, "docs/readme", "v1")
		mustWrite(t, s, "docs/readme", "v2")
		mustWrite(t, s, "docs/readme", "v3")
	}
	withDeleted := func(t *testing.T, s *llmd.Store) {
		mustWrite(t, s, "docs/active", "active")
		mustWrite(t, s, "docs/deleted", "deleted")
		mustDelete(t, s, "docs/deleted")
	}
	// Write in reverse alphabetical order with distinct timestamps so
	// time order (newest first) differs from path order.
	timed := func(t *testing.T, s *llmd.Store) {
		mustWrite(t, s, "notes/c", "c")
		time.Sleep(2 * time.Millisecond)
		mustWrite(t, s, "notes/b", "b")
		time.Sleep(2 * time.Millisecond)
		mustWrite(t, s, "notes/a", "a")
	}

	tests := []struct {
		name         string
		setup        func(t *testing.T, s *llmd.Store)
		opts         documents.ListOptions
		wantPaths    []string // in order
		wantVersions []int    // in order; nil skips the check
	}{
		{
			name:      "all documents sorted by path",
			setup:     three,
			wantPaths: []string{"docs/api", "docs/readme", "notes/todo"},
		},
		{
			name:      "prefix",
			setup:     three,
			opts:      documents.ListOptions{Prefix: "docs/"},
			wantPaths: []string{"docs/api", "docs/readme"},
		},
		{
			name:         "only latest version",
			setup:        versioned,
			wantPaths:    []string{"docs/readme"},
			wantVersions: []int{3},
		},
		{
			name:      "excludes deleted by default",
			setup:     withDeleted,
			wantPaths: []string{"docs/active"},
		},
		{
			name:      "include deleted",
			setup:     withDeleted,
			opts:      documents.ListOptions{IncludeDeleted: true},
			wantPaths: []string{"docs/active", "docs/deleted"},
		},
		{
			name:      "sort by time newest first",
			setup:     timed,
			opts:      documents.ListOptions{Sort: "time"},
			wantPaths: []string{"notes/a", "notes/b", "notes/c"},
		},
		{
			name:      "default sort is by path",
			setup:     timed,
			wantPaths: []string{"notes/a", "notes/b", "notes/c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			tt.setup(t, s)

			list, err := s.Documents.List(t.Context(), tt.opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tt.opts, err)
			}

			paths := make([]string, len(list))
			versions := make([]int, len(list))
			for i, info := range list {
				paths[i] = info.Path
				versions[i] = info.Version
			}
			if !slices.Equal(paths, tt.wantPaths) {
				t.Errorf("List(%+v) paths = %v, want %v", tt.opts, paths, tt.wantPaths)
			}
			if tt.wantVersions != nil && !slices.Equal(versions, tt.wantVersions) {
				t.Errorf("List(%+v) versions = %v, want %v", tt.opts, versions, tt.wantVersions)
			}
		})
	}
}
