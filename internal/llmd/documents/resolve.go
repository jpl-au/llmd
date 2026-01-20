package documents

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/hash"
	"github.com/jpl-au/llmd/internal/llmd/meta"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// Resolve finds a document by value, checking in order:
// 1. Filesystem (if file exists)
// 2. llmd path
// 3. llmd key (if value is 9 chars)
//
// All checks run in parallel. Returns first match with priority: fs > path > key.
func (d *Documents) Resolve(ctx context.Context, value string) (*document.Document, error) {
	var wg sync.WaitGroup
	var fsDoc, pathDoc, keyDoc *document.Document

	// Check filesystem
	wg.Go(func() {
		info, err := os.Stat(value)
		if err != nil || info.IsDir() {
			return
		}
		content, err := os.ReadFile(value)
		if err != nil {
			return
		}
		s := string(content)
		fsDoc = &document.Document{
			Path:     value,
			Content:  s,
			Hash:     hash.Blake2b(s),
			Meta:     meta.Compute(s),
			MIME:     mime,
			Source:   "filesystem",
			Resolved: document.ResolvedFile,
		}
	})

	// Check llmd path
	wg.Go(func() {
		doc, err := d.Read(ctx, value)
		if err != nil && !errors.Is(err, ErrDeleted) {
			return
		}
		pathDoc = doc
	})

	// Check llmd key (only if 9 chars)
	wg.Go(func() {
		if len(value) != 9 {
			return
		}
		doc, err := d.ReadByKey(ctx, value)
		if err != nil && !errors.Is(err, ErrDeleted) {
			return
		}
		keyDoc = doc
	})

	wg.Wait()

	// Return by priority: fs > path > key
	if fsDoc != nil {
		return fsDoc, nil
	}
	if pathDoc != nil {
		return pathDoc, nil
	}
	if keyDoc != nil {
		return keyDoc, nil
	}
	return nil, ErrNotFound
}
