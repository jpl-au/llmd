// mirror.go dumps all documents to the filesystem so editors and AI
// agents can reference them. Files are written preserving the document
// path structure, skipping unchanged content. Stale files not backed
// by a current document are removed.
//
// All filesystem access is confined to the mirror directory via
// os.OpenRoot, preventing path traversal and symlink escapes.

package bulk

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// MirrorResult contains the counts from a mirror operation.
type MirrorResult struct {
	Wrote   int
	Skipped int
	Removed int
}

// Mirror writes all documents matching prefix to dir as .md files.
// Unchanged files are skipped. Files under dir that no longer
// correspond to a document are removed.
func (b *Bulk) Mirror(ctx context.Context, prefix, dir string) (*MirrorResult, error) {
	docs, err := b.docs.List(ctx, documents.ListOptions{Prefix: prefix})
	if err != nil {
		return nil, err
	}

	// Ensure the mirror directory exists before opening a root on it.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating mirror directory: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening root %s: %w", dir, err)
	}
	defer root.Close()

	keep := make(map[string]bool)
	result := &MirrorResult{}

	for _, doc := range docs {
		content, err := b.docs.Read(ctx, doc.Path, documents.ReadOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", doc.Path, err)
		}

		rel := relFS(doc.Path)
		keep[rel] = true

		// Skip if file already has identical content.
		existing, err := root.ReadFile(rel)
		if err == nil && bytes.Equal(existing, []byte(content.Content)) {
			result.Skipped++
			continue
		}

		if d := filepath.Dir(rel); d != "." {
			if err := root.MkdirAll(d, 0755); err != nil {
				return nil, fmt.Errorf("creating directory: %w", err)
			}
		}
		if err := root.WriteFile(rel, []byte(content.Content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", rel, err)
		}
		result.Wrote++
	}

	// Only the subtree the prefix covers is pruned: a prefix pull must
	// not remove mirrored files of live documents outside it.
	removed, err := cleanStale(root, keep, strings.TrimSuffix(prefix, "/"))
	result.Removed = removed
	if err != nil {
		return nil, fmt.Errorf("removing stale files: %w", err)
	}
	return result, nil
}

// cleanStale removes files under root/scope that are not in the keep
// set, then removes any directories left empty. scope is a directory
// relative to root ("" for the whole tree). Returns the number of files
// removed and the first error met while walking or removing a file.
func cleanStale(root *os.Root, keep map[string]bool, scope string) (int, error) {
	var removed int

	start := "."
	if scope != "" {
		start = scope
		if _, err := fs.Stat(root.FS(), start); err != nil {
			return 0, nil // nothing mirrored under the prefix yet
		}
	}

	// First pass: remove stale files.
	err := fs.WalkDir(root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == "." {
			return nil
		}
		if !keep[p] {
			if err := root.Remove(p); err != nil {
				return fmt.Errorf("removing %s: %w", p, err)
			}
			removed++
		}
		return nil
	})
	if err != nil {
		return removed, err
	}

	// Second pass: remove empty directories, deepest first so that a
	// parent is emptied before it is tried. Remove fails on a directory
	// that still has contents, which is exactly the signal to keep it,
	// so that error is deliberately ignored.
	var dirs []string
	err = fs.WalkDir(root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != "." {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return removed, err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = root.Remove(dirs[i])
	}

	return removed, nil
}
