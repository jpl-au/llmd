package bulk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/hash"
)

// ImportResult contains the results of an import operation.
type ImportResult struct {
	Created []string // New documents added to store
	Updated []string // Existing documents got new version
	Skipped []string // Content identical, no change made
	Errors  []ImportError
}

// ImportError records an error for a specific path.
type ImportError struct {
	Path string
	Err  error
}

// importStatus indicates the result of importing a single file.
type importStatus int

const (
	statusCreated importStatus = iota
	statusUpdated
	statusSkipped
)

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
		storePath, status, err := b.importFile(ctx, path, "", opts)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Err: err})
		} else {
			b.recordStatus(result, storePath, status)
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

		storePath, status, err := b.importFile(ctx, p, base, opts)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: p, Err: err})
		} else {
			b.recordStatus(result, storePath, status)
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

func (b *Bulk) recordStatus(result *ImportResult, path string, status importStatus) {
	switch status {
	case statusCreated:
		result.Created = append(result.Created, path)
	case statusUpdated:
		result.Updated = append(result.Updated, path)
	case statusSkipped:
		result.Skipped = append(result.Skipped, path)
	}
}

func (b *Bulk) importFile(ctx context.Context, path, base string, opts ImportOptions) (string, importStatus, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
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

	contentStr := string(content)
	contentHash := hash.XXH3(contentStr)

	// Check if document already exists
	exists, err := b.docs.Exists(ctx, storePath)
	if err != nil {
		return "", 0, err
	}

	if exists && !opts.Force {
		// Read current version to compare hash
		doc, err := b.docs.Read(ctx, storePath)
		if err != nil {
			return "", 0, err
		}
		if doc.Hash == contentHash {
			return storePath, statusSkipped, nil
		}
	}

	if opts.DryRun {
		if exists {
			return storePath, statusUpdated, nil
		}
		return storePath, statusCreated, nil
	}

	_, err = b.docs.Write(ctx, storePath, contentStr, documents.WriteOptions{
		Origin: opts.Origin,
	})
	if err != nil {
		return "", 0, err
	}

	if exists {
		return storePath, statusUpdated, nil
	}
	return storePath, statusCreated, nil
}
