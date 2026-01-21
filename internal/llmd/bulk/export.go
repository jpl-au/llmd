package bulk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// If path ends with /, exports all docs with that prefix.
// Otherwise exports a single document.
func (b *Bulk) Export(ctx context.Context, path, dest string, opts ExportOptions) (*ExportResult, error) {
	result := &ExportResult{}

	if strings.HasSuffix(path, "/") {
		// Export multiple docs by prefix
		docs, err := b.docs.List(ctx, documents.ListOptions{Prefix: path})
		if err != nil {
			return nil, err
		}

		for _, doc := range docs {
			// Determine filesystem path
			relPath := strings.TrimPrefix(doc.Path, path)
			fsPath := filepath.Join(dest, relPath+".md")

			if err := b.exportOne(ctx, doc.Path, fsPath, opts); err != nil {
				if os.IsExist(err) {
					result.Skipped = append(result.Skipped, fsPath)
				} else {
					result.Errors = append(result.Errors, ExportError{Path: doc.Path, Err: err})
				}
			} else {
				result.Exported = append(result.Exported, fsPath)
			}
		}

		return result, nil
	}

	// Single document export
	fsPath := dest
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		// dest is a directory, use doc path as filename
		fsPath = filepath.Join(dest, filepath.Base(path)+".md")
	}

	if err := b.exportOne(ctx, path, fsPath, opts); err != nil {
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

func (b *Bulk) exportOne(ctx context.Context, src, dest string, opts ExportOptions) error {
	// Read document
	doc, err := b.docs.Read(ctx, src, documents.ReadOptions{Version: opts.Version})
	if err != nil {
		return err
	}

	// Check if file exists
	if !opts.Force {
		if _, err := os.Stat(dest); err == nil {
			return os.ErrExist
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Write file
	return os.WriteFile(dest, []byte(doc.Content), 0644)
}
