package bulk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// ImportResult contains the results of an import operation.
type ImportResult struct {
	Imported []string // Paths that were imported
	Skipped  []string // Paths that were skipped (unchanged)
	Errors   []ImportError
}

// ImportError records an error for a specific path.
type ImportError struct {
	Path string
	Err  error
}

// Import imports markdown files from filesystem into the store.
func (b *Bulk) Import(ctx context.Context, path string, opts ImportOptions) (*ImportResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	result := &ImportResult{}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		doc, err := b.importFile(ctx, path, "", opts)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Err: err})
		} else if doc != nil {
			result.Imported = append(result.Imported, doc.Path)
		} else {
			result.Skipped = append(result.Skipped, path)
		}
		return result, nil
	}

	// Directory walk
	base := path
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files/directories unless requested
		name := d.Name()
		if !opts.Hidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Only import markdown files
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}

		doc, err := b.importFile(ctx, p, base, opts)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: p, Err: err})
		} else if doc != nil {
			result.Imported = append(result.Imported, doc.Path)
		} else {
			result.Skipped = append(result.Skipped, p)
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

func (b *Bulk) importFile(ctx context.Context, path, base string, opts ImportOptions) (*document.Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Determine store path
	var storePath string
	if opts.Flatten || base == "" {
		// Just use filename without .md extension
		storePath = strings.TrimSuffix(filepath.Base(path), ".md")
	} else {
		// Preserve relative path structure
		rel, err := filepath.Rel(base, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		// Remove .md extension and convert to forward slashes
		storePath = strings.TrimSuffix(rel, ".md")
		storePath = filepath.ToSlash(storePath)
	}

	// Add prefix if specified
	if opts.Prefix != "" {
		storePath = strings.TrimSuffix(opts.Prefix, "/") + "/" + storePath
	}

	if opts.DryRun {
		return &document.Document{Path: storePath}, nil
	}

	return b.docs.Write(ctx, storePath, string(content), documents.WriteOptions{
		WriteContext: opts.WriteContext,
	})
}
