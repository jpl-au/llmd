// mirror.go dumps all documents to the filesystem as .md files so editors
// and AI agents can reference them (e.g. @ mentions in Claude Code).
//
// Files are written to .llmd/mirror/ preserving the document path structure.
// This is a one-way push — the filesystem copy is disposable and regenerated
// from the store each time. Use import/export for bidirectional operations.
//
// Usage:
//
//	llmd mirror              Mirror all documents
//	llmd mirror <prefix>     Mirror documents under a prefix

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

const mirrorDir = ".llmd/mirror"

func mirror(ctx sdk.Context, args []string) (sdk.Response, error) {
	var prefix string
	if len(args) > 0 {
		prefix = args[0]
	}

	docs, err := sdk.API.List(prefix, sdk.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}

	// Track which files we write so we can clean up stale ones.
	written := make(map[string]bool)
	var wrote, skipped, removed int

	for _, doc := range docs {
		content, err := sdk.API.Read(doc.Path, 0)
		if err != nil {
			return nil, fmt.Errorf("mirror: reading %s: %w", doc.Path, err)
		}

		fsPath := docToPath(doc.Path)
		written[fsPath] = true

		// Skip if file already has identical content.
		existing, err := os.ReadFile(fsPath)
		if err == nil && bytes.Equal(existing, content) {
			skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fsPath), 0755); err != nil {
			return nil, fmt.Errorf("mirror: creating directory: %w", err)
		}
		if err := os.WriteFile(fsPath, content, 0644); err != nil {
			return nil, fmt.Errorf("mirror: writing %s: %w", fsPath, err)
		}
		wrote++
	}

	// Remove stale files not in the current document set.
	removed = cleanStale(mirrorDir, written)

	var parts []string
	if wrote > 0 {
		parts = append(parts, fmt.Sprintf("wrote %d", wrote))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("unchanged %d", skipped))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("removed %d stale", removed))
	}
	if len(parts) == 0 {
		return sdk.Text("Nothing to mirror"), nil
	}

	return sdk.Text(fmt.Sprintf("Mirrored to %s/ (%s)", mirrorDir, strings.Join(parts, ", "))), nil
}

// docToPath converts a document path to a filesystem path under mirrorDir.
// Adds .md extension if the path doesn't already have one.
func docToPath(docPath string) string {
	if filepath.Ext(docPath) == "" {
		docPath += ".md"
	}
	return filepath.Join(mirrorDir, docPath)
}

// cleanStale removes files under dir that aren't in the keep set.
// Returns the number of files removed. Also removes empty directories.
func cleanStale(dir string, keep map[string]bool) int {
	var removed int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !keep[path] {
			os.Remove(path)
			removed++
		}
		return nil
	})
	// Clean empty directories (walk bottom-up by trying to remove all dirs).
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == dir {
			return nil
		}
		os.Remove(path) // only succeeds if empty
		return nil
	})
	return removed
}
