package documents

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/hash"
	"github.com/jpl-au/llmd/internal/llmd/meta"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// parsePathVersion splits "path:version" into path and optional version.
// Returns path and nil if no version suffix, or path and version pointer if present.
// The split is on the last colon to support paths containing colons.
func parsePathVersion(value string) (string, *int) {
	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return value, nil
	}
	suffix := value[idx+1:]
	v, err := strconv.Atoi(suffix)
	if err != nil {
		// Not a valid version number, treat whole string as path
		return value, nil
	}
	return value[:idx], &v
}

// Resolve finds a document by value, checking in order:
// 1. Filesystem (if file exists)
// 2. llmd path (supports path:version syntax)
// 3. llmd key (if value is 9 chars)
//
// All checks run in parallel. Returns first match with priority: fs > path > key.
func (d *Documents) Resolve(ctx context.Context, value string) (*document.Document, error) {
	// Parse path:version syntax
	path, version := parsePathVersion(value)

	var wg sync.WaitGroup
	var fsDoc, pathDoc, keyDoc *document.Document

	// Check filesystem (only if no version specified)
	wg.Go(func() {
		if version != nil {
			return // version syntax means llmd document
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return
		}
		s := string(content)
		fsDoc = &document.Document{
			Path:     path,
			Content:  s,
			Hash:     hash.XXH3(s),
			Meta:     meta.Compute(s),
			MIME:     mime,
			Source:   "filesystem",
			Resolved: document.ResolvedFile,
		}
	})

	// Check llmd path
	wg.Go(func() {
		var opts ReadOptions
		if version != nil {
			opts.Version = version
		}
		doc, err := d.Read(ctx, path, opts)
		if err != nil && !errors.Is(err, ErrDeleted) {
			return
		}
		pathDoc = doc
	})

	// Check llmd key (only if 9 chars and no version)
	wg.Go(func() {
		if version != nil || len(path) != 9 {
			return
		}
		doc, err := d.ReadByKey(ctx, path)
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
