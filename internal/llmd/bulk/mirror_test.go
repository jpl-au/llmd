package bulk_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// TestMirror covers the mirror pull: the first pull writes every
// document, a repeat pull skips unchanged files, documents deleted or
// renamed in the store disappear from the mirror, and a prefix pull
// only touches the subtree the prefix covers. Each case writes docs to
// a fresh store, optionally seeds the mirror directory with a full
// pull, applies change to the store, then measures one pull with
// prefix. Paths in existing, wantFiles and wantAbsent are relative to
// the mirror directory.
func TestMirror(t *testing.T) {
	tests := []struct {
		name       string
		docs       []doc             // written to the store in order
		existing   map[string]string // files planted in the mirror directory after the seed pull
		seed       bool              // run a full pull first so the directory mirrors docs
		change     func(t *testing.T, s *llmd.Store)
		subdir     string // mirror into this not-yet-existing subdirectory of the temp dir
		prefix     string
		want       bulk.MirrorResult
		wantFiles  map[string]string
		wantAbsent []string
	}{
		{
			name:      "first pull writes everything",
			docs:      []doc{{"readme", "# Hello"}, {"docs/guide", "guide"}},
			want:      bulk.MirrorResult{Wrote: 2},
			wantFiles: map[string]string{"readme.md": "# Hello", "docs/guide.md": "guide"},
		},
		{
			name:      "second pull skips unchanged",
			docs:      []doc{{"readme", "# Hello"}, {"docs/guide", "guide"}},
			seed:      true,
			want:      bulk.MirrorResult{Skipped: 2},
			wantFiles: map[string]string{"readme.md": "# Hello", "docs/guide.md": "guide"},
		},
		{
			name: "changed document rewritten",
			docs: []doc{{"readme", "v1"}, {"docs/guide", "guide"}},
			seed: true,
			change: func(t *testing.T, s *llmd.Store) {
				if _, err := s.Documents.Write(t.Context(), "readme", "v2", testWriteOpts()); err != nil {
					t.Fatalf("Write(readme) error = %v", err)
				}
			},
			want:      bulk.MirrorResult{Wrote: 1, Skipped: 1},
			wantFiles: map[string]string{"readme.md": "v2", "docs/guide.md": "guide"},
		},
		{
			name: "deleted document removed",
			docs: []doc{{"readme", "# Hello"}, {"docs/guide", "guide"}},
			seed: true,
			change: func(t *testing.T, s *llmd.Store) {
				if err := s.Documents.Delete(t.Context(), "docs/guide", documents.DeleteOptions{Origin: testWriteOpts().Origin}); err != nil {
					t.Fatalf("Delete(docs/guide) error = %v", err)
				}
			},
			want:       bulk.MirrorResult{Skipped: 1, Removed: 1},
			wantFiles:  map[string]string{"readme.md": "# Hello"},
			wantAbsent: []string{"docs/guide.md", "docs"},
		},
		{
			name: "renamed document removes old file",
			docs: []doc{{"old", "content"}},
			seed: true,
			change: func(t *testing.T, s *llmd.Store) {
				if err := s.Documents.Move(t.Context(), "old", "new", documents.MoveOptions{Origin: testWriteOpts().Origin}); err != nil {
					t.Fatalf("Move(old, new) error = %v", err)
				}
			},
			want:       bulk.MirrorResult{Wrote: 1, Removed: 1},
			wantFiles:  map[string]string{"new.md": "content"},
			wantAbsent: []string{"old.md"},
		},
		{
			// Only docs/ is written and pruned: the stale file under docs/
			// goes, the one under notes/ and the live notes document stay.
			name:     "prefix pull leaves other subtrees alone",
			docs:     []doc{{"docs/guide", "v1"}, {"notes/todo", "todo"}},
			existing: map[string]string{"docs/stale.md": "stale", "notes/stale.md": "stale"},
			seed:     true,
			change: func(t *testing.T, s *llmd.Store) {
				if _, err := s.Documents.Write(t.Context(), "docs/guide", "v2", testWriteOpts()); err != nil {
					t.Fatalf("Write(docs/guide) error = %v", err)
				}
			},
			prefix: "docs/",
			want:   bulk.MirrorResult{Wrote: 1, Removed: 1},
			wantFiles: map[string]string{
				"docs/guide.md":  "v2",
				"notes/todo.md":  "todo",
				"notes/stale.md": "stale",
			},
			wantAbsent: []string{"docs/stale.md"},
		},
		{
			name:      "prefix pull into new directory",
			docs:      []doc{{"docs/guide", "guide"}, {"notes/todo", "todo"}},
			subdir:    "mirror",
			prefix:    "docs/",
			want:      bulk.MirrorResult{Wrote: 1},
			wantFiles: map[string]string{"docs/guide.md": "guide"},
			wantAbsent: []string{
				"notes/todo.md",
			},
		},
		{
			// Nothing under the prefix in the store or on disk: no
			// subtree to prune, and the pull is a no-op.
			name:       "prefix with nothing mirrored",
			docs:       []doc{{"notes/todo", "todo"}},
			prefix:     "docs/",
			wantAbsent: []string{"notes/todo.md", "docs"},
		},
		{
			name:      "nested directories created",
			docs:      []doc{{"a/b/c/deep", "deep"}},
			want:      bulk.MirrorResult{Wrote: 1},
			wantFiles: map[string]string{"a/b/c/deep.md": "deep"},
		},
		{
			// Removing the only document under a/b/c/ leaves three empty
			// directories behind; all of them should go in one pull.
			name: "nested empty directories removed",
			docs: []doc{{"a/b/c/deep", "deep"}, {"readme", "# Hello"}},
			seed: true,
			change: func(t *testing.T, s *llmd.Store) {
				if err := s.Documents.Delete(t.Context(), "a/b/c/deep", documents.DeleteOptions{Origin: testWriteOpts().Origin}); err != nil {
					t.Fatalf("Delete(a/b/c/deep) error = %v", err)
				}
			},
			want:       bulk.MirrorResult{Skipped: 1, Removed: 1},
			wantFiles:  map[string]string{"readme.md": "# Hello"},
			wantAbsent: []string{"a/b/c/deep.md", "a/b/c", "a/b", "a"},
		},
		{
			name: "no documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			for _, d := range tt.docs {
				if _, err := s.Documents.Write(ctx, d.path, d.content, testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) error = %v", d.path, err)
				}
			}

			dir := t.TempDir()
			if tt.subdir != "" {
				dir = filepath.Join(dir, tt.subdir)
			}
			if tt.seed {
				if _, err := s.Bulk.Mirror(ctx, "", dir); err != nil {
					t.Fatalf("Mirror(\"\") seed error = %v", err)
				}
			}
			for name, content := range tt.existing {
				p := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatalf("MkdirAll(%q) error = %v", name, err)
				}
				if err := os.WriteFile(p, []byte(content), 0644); err != nil {
					t.Fatalf("WriteFile(%q) error = %v", name, err)
				}
			}
			if tt.change != nil {
				tt.change(t, s)
			}

			got, err := s.Bulk.Mirror(ctx, tt.prefix, dir)
			if err != nil {
				t.Fatalf("Mirror(%q) error = %v", tt.prefix, err)
			}
			if *got != tt.want {
				t.Errorf("Mirror(%q) = %+v, want %+v", tt.prefix, *got, tt.want)
			}

			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				t.Errorf("Stat(mirror dir) = %v, %v, want directory", info, err)
			}
			for name, want := range tt.wantFiles {
				content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
				if err != nil {
					t.Errorf("ReadFile(%q) error = %v", name, err)
					continue
				}
				if string(content) != want {
					t.Errorf("%s content = %q, want %q", name, string(content), want)
				}
			}
			for _, name := range tt.wantAbsent {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil {
					t.Errorf("%s exists, want absent", name)
				}
			}
		})
	}
}
