// mirror.go dumps all documents to the filesystem so editors and AI
// agents can reference them. Files are written preserving the document
// path structure, skipping unchanged content. Stale files not backed
// by a current document are removed.

package bulk

import (
	"bytes"
	"context"
	"fmt"
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

	written := make(map[string]bool)
	result := &MirrorResult{}

	for _, doc := range docs {
		content, err := b.docs.Read(ctx, doc.Path, documents.ReadOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", doc.Path, err)
		}

		fsPath := docToFSPath(dir, doc.Path)
		written[fsPath] = true

		// Skip if file already has identical content.
		existing, err := os.ReadFile(fsPath)
		if err == nil && bytes.Equal(existing, []byte(content.Content)) {
			result.Skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fsPath), 0755); err != nil {
			return nil, fmt.Errorf("creating directory: %w", err)
		}
		if err := os.WriteFile(fsPath, []byte(content.Content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", fsPath, err)
		}
		result.Wrote++
	}

	result.Removed = cleanStale(dir, written)
	return result, nil
}

// docToFSPath converts a document path to a filesystem path under dir.
// Adds .md extension if the path has no extension.
func docToFSPath(dir, docPath string) string {
	if filepath.Ext(docPath) == "" {
		docPath += ".md"
	}
	return filepath.Join(dir, docPath)
}

// cleanStale removes files under dir that are not in the keep set.
// Also removes empty directories left behind. Returns the number of
// files removed.
func cleanStale(dir string, keep map[string]bool) int {
	var removed int
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !keep[path] {
			os.Remove(path)
			removed++
		}
		return nil
	}); err != nil {
		return removed
	}
	// Remove empty directories (only succeeds if empty).
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == dir {
			return nil
		}
		os.Remove(path)
		return nil
	}); err != nil {
		return removed
	}
	return removed
}
