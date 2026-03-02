package bulk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/hash"
	docpath "github.com/jpl-au/llmd/internal/path"
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

// importStatus indicates the outcome of importing a single file. Each
// file is classified after comparing its content hash to the existing
// store document (if any). The status determines which result bucket
// (Created, Updated, Skipped) the path lands in.
type importStatus int

const (
	// statusCreated means the document did not exist and was newly added.
	statusCreated importStatus = iota

	// statusUpdated means the document existed but had different content,
	// so a new version was written.
	statusUpdated

	// statusSkipped means the document existed with identical content
	// (matching XXH3 hash), so no write was needed.
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

// recordStatus appends a path to the appropriate result bucket based
// on the import outcome.
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

// importFile imports a single file. It reads the file, computes its
// store path (based on relative position, prefix, and flatten settings),
// compares the XXH3 hash to any existing document, and writes a new
// version if the content has changed. Returns the store path, the
// import status, and any error.
func (b *Bulk) importFile(ctx context.Context, path, base string, opts ImportOptions) (string, importStatus, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}

	// Determine store path. Normalise handles backslash conversion,
	// .md stripping, and traversal validation.
	var raw string
	if opts.Flatten || base == "" {
		raw = filepath.Base(path)
	} else {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		raw = rel
	}

	storePath, err := docpath.Normalise(raw)
	if err != nil {
		return "", 0, fmt.Errorf("normalise %q: %w", raw, err)
	}

	// Add prefix if specified
	if opts.Prefix != "" {
		storePath = strings.TrimSuffix(opts.Prefix, "/") + "/" + storePath
	}

	// Normalise line endings so content is consistent regardless of OS.
	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
	contentStr = strings.ReplaceAll(contentStr, "\r", "\n")
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
