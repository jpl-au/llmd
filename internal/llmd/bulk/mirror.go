// mirror.go dumps all documents to the filesystem so editors and AI
// agents can reference them. Files are written preserving the document
// path structure, skipping unchanged content. Stale files not backed
// by a current document are removed.
//
// All filesystem access is confined to the mirror directory via
// os.OpenRoot, preventing path traversal and symlink escapes.

package bulk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
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

	// Ensure the mirror directory exists before opening a root on it.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating mirror directory: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening root %s: %w", dir, err)
	}
	defer root.Close()

	// The pull rewrites the manifest entries it covers, so a document
	// deleted from the store since the last pull drops out of it too.
	m, err := readManifest(dir)
	if err != nil {
		return nil, err
	}
	for p := range m.Docs {
		if strings.HasPrefix(p, prefix) {
			delete(m.Docs, p)
		}
	}

	keep := make(map[string]bool)
	result := &MirrorResult{}

	for _, doc := range docs {
		content, err := b.docs.Read(ctx, doc.Path, documents.ReadOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", doc.Path, err)
		}
		m.Docs[doc.Path] = content.Hash

		rel := relFS(doc.Path)
		// The stale pass walks an fs.FS, whose paths always use
		// slashes, so the keep set must too or every mirrored file
		// under a directory is removed on Windows.
		keep[filepath.ToSlash(rel)] = true

		// Skip if file already has identical content.
		existing, err := root.ReadFile(rel)
		if err == nil && bytes.Equal(existing, []byte(content.Content)) {
			result.Skipped++
			continue
		}

		if d := filepath.Dir(rel); d != "." {
			if err := root.MkdirAll(d, 0755); err != nil {
				return nil, fmt.Errorf("creating directory: %w", err)
			}
		}
		if err := root.WriteFile(rel, []byte(content.Content), 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", rel, err)
		}
		result.Wrote++
	}

	// Tags and links of every document, whatever the prefix: the
	// sidecar describes the whole store, keyed by store path as the
	// mirror's files are.
	all, err := b.docs.List(ctx, documents.ListOptions{})
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(all))
	for i, d := range all {
		paths[i] = d.Path
	}
	if m.Meta, err = b.collectMeta(ctx, paths, func(p string) (string, bool) { return p, true }); err != nil {
		return nil, err
	}
	if err := writeMeta(root, m.Meta); err != nil {
		return nil, err
	}
	keep[metaFile] = true

	// Only the subtree the prefix covers is pruned: a prefix pull must
	// not remove mirrored files of live documents outside it.
	removed, err := cleanStale(root, keep, strings.TrimSuffix(prefix, "/"))
	result.Removed = removed
	if err != nil {
		return nil, fmt.Errorf("removing stale files: %w", err)
	}
	if err := m.write(dir); err != nil {
		return nil, err
	}
	return result, nil
}

// MirrorPushOptions configures a mirror push.
type MirrorPushOptions struct {
	core.Origin
	Prefix string // Only push the files under this path
}

// MirrorPushResult contains the outcome of a mirror push.
type MirrorPushResult struct {
	ImportResult
	Deleted []string // Documents deleted because their file was removed
}

// MirrorPush imports the .md files under dir back into the store, then
// deletes the documents whose mirror file has been removed since the
// last pull or push. A prefix limits both to the files under that
// path: pull puts docs/a at <dir>/docs/a.md, so a push of docs reads
// <dir>/docs and keeps docs/ in each path.
//
// Only documents in the manifest are deleted, so a document written
// to the store since the last pull, which has no file yet, is left
// alone. A document changed in the store since its file was removed
// is kept as well; the next pull writes its file back. Deletes are
// soft, so llmd restore brings a document back.
func (b *Bulk) MirrorPush(ctx context.Context, dir string, opts MirrorPushOptions) (*MirrorPushResult, error) {
	src := dir
	if prefix := strings.Trim(opts.Prefix, "/"); prefix != "" {
		src = filepath.Join(dir, filepath.FromSlash(prefix))
	}
	r, err := b.Import(ctx, src, ImportOptions{Origin: opts.Origin, Prefix: opts.Prefix, skipMeta: true})
	if err != nil {
		return nil, err
	}
	result := &MirrorPushResult{ImportResult: *r}

	m, err := readManifest(dir)
	if err != nil {
		return nil, err
	}

	for p, h := range m.Docs {
		if !underDir(p, opts.Prefix) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, relFS(p))); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if b.docs.Check(ctx, p) != nil {
			delete(m.Docs, p) // already deleted or moved in the store
			continue
		}
		doc, err := b.docs.Read(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		if doc.Hash != h {
			continue
		}
		if err := b.docs.Delete(ctx, p, documents.DeleteOptions{Origin: opts.Origin}); err != nil {
			return nil, fmt.Errorf("deleting %s: %w", p, err)
		}
		delete(m.Docs, p)
		result.Deleted = append(result.Deleted, p)
	}
	slices.Sort(result.Deleted)

	// Every file pushed now matches its document.
	for _, paths := range [][]string{r.Created, r.Updated, r.Skipped} {
		for _, p := range paths {
			doc, err := b.docs.Read(ctx, p)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", p, err)
			}
			m.Docs[p] = doc.Hash
		}
	}

	// Tags and links: what changed in the sidecar since the base.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening root %s: %w", dir, err)
	}
	defer root.Close()
	file, err := readMeta(root)
	if err != nil {
		return nil, err
	}
	var failed []ImportError
	result.Meta, failed = b.applyMeta(ctx, file, m.Meta, func(p string) bool { return underDir(p, opts.Prefix) },
		func(p string) string { return p }, opts.Origin)
	result.Errors = append(result.Errors, failed...)

	// The new base: the file for the paths pushed, the old base for the
	// rest, which this push did not apply.
	base := meta{}
	for p, dm := range m.Meta {
		if !underDir(p, opts.Prefix) {
			base[p] = dm
		}
	}
	for p, dm := range file {
		if underDir(p, opts.Prefix) {
			base[p] = dm
		}
	}
	m.Meta = base

	if err := m.write(dir); err != nil {
		return nil, err
	}
	return result, nil
}

// cleanStale removes files under root/scope that are not in the keep
// set, then removes any directories left empty. scope is a directory
// relative to root ("" for the whole tree). Returns the number of files
// removed and the first error met while walking or removing a file.
func cleanStale(root *os.Root, keep map[string]bool, scope string) (int, error) {
	var removed int

	start := "."
	if scope != "" {
		start = scope
		if _, err := fs.Stat(root.FS(), start); err != nil {
			return 0, nil // nothing mirrored under the prefix yet
		}
	}

	// First pass: remove stale files.
	err := fs.WalkDir(root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == "." {
			return nil
		}
		if !keep[p] {
			if err := root.Remove(p); err != nil {
				return fmt.Errorf("removing %s: %w", p, err)
			}
			removed++
		}
		return nil
	})
	if err != nil {
		return removed, err
	}

	// Second pass: remove empty directories, deepest first so that a
	// parent is emptied before it is tried. Remove fails on a directory
	// that still has contents, which is exactly the signal to keep it,
	// so that error is deliberately ignored.
	var dirs []string
	err = fs.WalkDir(root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != "." {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return removed, err
	}
	for _, dir := range slices.Backward(dirs) {
		_ = root.Remove(dir)
	}

	return removed, nil
}
