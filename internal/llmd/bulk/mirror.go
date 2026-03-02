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

	result.Removed = cleanStale(root, keep)
	return result, nil
}

// cleanStale removes files under root that are not in the keep set.
// Also removes empty directories left behind. Returns the number of
// files removed.
func cleanStale(root *os.Root, keep map[string]bool) int {
	var removed int

	// First pass: remove stale files.
	fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "." {
			return nil
		}
		if !keep[p] {
			root.Remove(p)
			removed++
		}
		return nil
	})

	// Second pass: remove empty directories (only succeeds if empty).
	fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == "." {
			return nil
		}
		root.Remove(p)
		return nil
	})

	return removed
}
