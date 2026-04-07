package documents

import (
	"context"
	"os"

	"github.com/jpl-au/llmd/internal/llmd/hash"
	"github.com/jpl-au/llmd/internal/llmd/meta"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// Resolve finds a document by identifier. The identifier can be a path,
// a key, or either with a :version suffix. Resolution is handled by the
// shared resolve package; this method adds the document-specific
// filesystem fallback for when the identifier points to a local file.
func (d *Documents) Resolve(ctx context.Context, value string) (*document.Document, error) {
	path, version, byKey := resolve.Identifier(ctx, value, d.KeyToPath)

	// Filesystem takes priority when no version is specified and the
	// identifier was not resolved via key lookup.
	if version == nil && !byKey {
		if fsDoc, err := d.readFilesystem(path); err == nil {
			return fsDoc, nil
		}
	}

	// Read from the store using the resolved path.
	var opts ReadOptions
	if version != nil {
		opts.Version = version
	}
	doc, err := d.Read(ctx, path, opts)
	if doc != nil {
		if byKey {
			doc.Resolved = document.ResolvedKey
		}
		return doc, err
	}

	return nil, err
}

// readFilesystem reads a document from the local filesystem.
func (d *Documents) readFilesystem(path string) (*document.Document, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, ErrNotFound
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := string(content)
	return &document.Document{
		Path:     path,
		Content:  s,
		Hash:     hash.XXH3(s),
		Meta:     meta.Compute(s),
		MIME:     mime,
		Source:   "filesystem",
		Resolved: document.ResolvedFile,
	}, nil
}

// KeyToPath translates a key to its document path. Used by the shared
// resolve package to detect key-based identifiers.
func (d *Documents) KeyToPath(ctx context.Context, key string) (string, error) {
	doc, err := d.ReadByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return doc.Path, nil
}
