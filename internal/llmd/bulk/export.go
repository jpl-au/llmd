package bulk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/line"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// ExportResult contains the results of an export operation.
type ExportResult struct {
	Exported []string // Filesystem paths that were written
	Skipped  []string // Paths skipped (already exist)
	Errors   []ExportError
}

// ExportError records an error for a specific path.
type ExportError struct {
	Path string
	Err  error
}

// Export exports documents from store to filesystem.
//
// All filesystem access is confined to the destination directory via
// os.OpenRoot, preventing path traversal and symlink escapes.
//
// Path modes:
//   - Single document: "docs/readme" exports one document
//   - Prefix (batch): "docs/" exports all documents under that prefix
//
// The prefix mode (path ending with /) exports all matching documents,
// preserving their relative paths under the destination directory.
// For example, exporting "api/" to "./out" would write:
//   - api/users    -> ./out/users.md
//   - api/v2/auth  -> ./out/v2/auth.md
func (b *Bulk) Export(ctx context.Context, path, dest string, opts ExportOptions) (*ExportResult, error) {
	result := &ExportResult{}

	if strings.HasSuffix(path, "/") {
		// Prefix export — root is the destination directory.
		root, err := os.OpenRoot(dest)
		if err != nil {
			return nil, fmt.Errorf("opening root %s: %w", dest, err)
		}
		defer root.Close()

		docs, err := b.docs.List(ctx, documents.ListOptions{Prefix: path})
		if err != nil {
			return nil, err
		}

		for _, doc := range docs {
			rel := relFS(strings.TrimPrefix(doc.Path, path))

			if err := b.exportOne(ctx, doc.Path, root, rel, opts); err != nil {
				if os.IsExist(err) {
					result.Skipped = append(result.Skipped, filepath.Join(dest, rel))
				} else {
					result.Errors = append(result.Errors, ExportError{Path: doc.Path, Err: err})
				}
			} else {
				result.Exported = append(result.Exported, filepath.Join(dest, rel))
			}
		}

		return result, nil
	}

	// Single document export — determine root directory and relative name.
	var rootDir, rel string
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		rootDir = dest
		rel = relFS(filepath.Base(path))
	} else {
		rootDir = filepath.Dir(dest)
		rel = filepath.Base(dest)
	}

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", rootDir, err)
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("opening root %s: %w", rootDir, err)
	}
	defer root.Close()

	fsPath := filepath.Join(rootDir, rel)
	if err := b.exportOne(ctx, path, root, rel, opts); err != nil {
		if os.IsExist(err) {
			result.Skipped = append(result.Skipped, fsPath)
		} else {
			result.Errors = append(result.Errors, ExportError{Path: path, Err: err})
		}
	} else {
		result.Exported = append(result.Exported, fsPath)
	}

	return result, nil
}

// exportOne exports a single document to a root-relative path. It reads
// the document content from the store, creates any necessary parent
// directories within the root, and writes the file. Respects the
// Overwrite option to avoid clobbering existing files.
func (b *Bulk) exportOne(ctx context.Context, src string, root *os.Root, rel string, opts ExportOptions) error {
	doc, err := b.docs.Read(ctx, src, documents.ReadOptions{Version: opts.Version})
	if err != nil {
		return err
	}

	if !opts.Overwrite {
		if _, err := root.Stat(rel); err == nil {
			return os.ErrExist
		}
	}

	if dir := filepath.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	return root.WriteFile(rel, []byte(line.Native(doc.Content)), 0644)
}

// relFS converts a document path to a root-relative filesystem path.
// Adds .md extension if the path has none.
func relFS(docPath string) string {
	if filepath.Ext(docPath) == "" {
		docPath += ".md"
	}
	return filepath.FromSlash(docPath)
}
