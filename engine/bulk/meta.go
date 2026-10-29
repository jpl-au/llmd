// meta.go carries tags and links alongside exported and mirrored
// documents, in one .meta.yaml beside the .md files. It is keyed by
// document path, relative to the export like the files are, and holds
// each document's tags and outgoing links:
//
//	docs/api:
//	  tags:
//	    - backend
//	  links:
//	    - to: docs/auth
//	      label: depends-on
//
// Entries are sorted, one per line, so two people tagging or linking
// different documents merge cleanly in git.

package bulk

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jpl-au/llmd/engine/links"
	"github.com/jpl-au/llmd/engine/tags"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// metaFile is the sidecar's name. It is hidden, so imports never take
// it for a document.
const metaFile = ".meta.yaml"

// meta maps a document path to its tags and outgoing links.
type meta map[string]docMeta

// docMeta is one document's entry.
type docMeta struct {
	Tags  []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
	Links []metaLink `yaml:"links,omitempty" json:"links,omitempty"`
}

// metaLink is an outgoing link.
type metaLink struct {
	To    string `yaml:"to" json:"to"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// MetaResult counts the tag and link changes an import or push made.
type MetaResult struct {
	Tagged   int // tags added
	Untagged int // tags removed
	Linked   int // links added
	Unlinked int // links removed
}

// collectMeta returns the tags and outgoing links of the documents at
// paths. rel maps a store path to its path in the sidecar, and reports
// false for a path outside the export, so links to documents that are
// not exported are left out.
func (b *Bulk) collectMeta(ctx context.Context, paths []string, rel func(string) (string, bool)) (meta, error) {
	m := meta{}
	for _, p := range paths {
		from, ok := rel(p)
		if !ok {
			continue
		}
		var dm docMeta
		ts, err := b.tags.List(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("listing tags of %s: %w", p, err)
		}
		for _, t := range ts {
			dm.Tags = append(dm.Tags, t.Value.Tag)
		}
		ls, err := b.links.List(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("listing links of %s: %w", p, err)
		}
		for _, l := range ls {
			if to, ok := rel(l.Value.To); ok {
				dm.Links = append(dm.Links, metaLink{To: to, Label: l.Value.Label})
			}
		}
		if len(dm.Tags) > 0 || len(dm.Links) > 0 {
			m[from] = dm.sorted()
		}
	}
	return m, nil
}

// sorted returns the entry with its tags and links in order, so the
// file is the same however the store returned them.
func (dm docMeta) sorted() docMeta {
	slices.Sort(dm.Tags)
	slices.SortFunc(dm.Links, func(a, b metaLink) int {
		if c := strings.Compare(a.To, b.To); c != 0 {
			return c
		}
		return strings.Compare(a.Label, b.Label)
	})
	return dm
}

// readMeta loads the sidecar from root. A missing file is empty.
func readMeta(root *os.Root) (meta, error) {
	data, err := root.ReadFile(metaFile)
	if errors.Is(err, fs.ErrNotExist) {
		return meta{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", metaFile, err)
	}
	m := meta{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("reading %s: %w", metaFile, err)
	}
	if m == nil {
		m = meta{}
	}
	return m, nil
}

// writeMeta saves the sidecar to root, or removes it when there is
// nothing to say. yaml sorts the document paths.
func writeMeta(root *os.Root, m meta) error {
	if len(m) == 0 {
		if err := root.Remove(metaFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", metaFile, err)
		}
		return nil
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	if err := root.WriteFile(metaFile, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", metaFile, err)
	}
	return nil
}

// applyMeta brings the store's tags and links in line with the
// sidecar for the paths in scope. store maps a sidecar path to its
// store path.
//
// Without a base, as for a plain import, it only adds: whatever the
// sidecar has and the store lacks. With a base, the sidecar as the
// last pull or push left it, it applies what changed in the sidecar
// since: added entries are added and removed ones removed, while a tag
// or link made in the store since the base is kept.
//
// A tag or link that cannot be applied, such as a link to a document
// that does not exist, is reported in the returned errors and the rest
// still apply.
func (b *Bulk) applyMeta(ctx context.Context, file, base meta, scope func(string) bool, store func(string) string, origin core.Origin) (MetaResult, []ImportError) {
	var r MetaResult
	var failed []ImportError
	fail := func(path string, err error) { failed = append(failed, ImportError{Path: path, Err: err}) }

	paths := slices.Sorted(maps.Keys(file))
	for p := range base {
		if _, ok := file[p]; !ok {
			paths = append(paths, p)
		}
	}
	for _, p := range paths {
		if !scope(p) {
			continue
		}
		from := store(p)
		want, was := file[p], base[p]

		have := map[string]bool{}
		if ts, err := b.tags.List(ctx, from); err == nil {
			for _, t := range ts {
				have[t.Value.Tag] = true
			}
		}
		for _, name := range want.Tags {
			if have[name] || (base != nil && slices.Contains(was.Tags, name)) {
				continue
			}
			if _, err := b.tags.Add(ctx, from, name, tags.Options{Origin: origin}); err != nil {
				fail(from, fmt.Errorf("tag %s: %w", name, err))
				continue
			}
			r.Tagged++
		}
		for _, name := range was.Tags {
			if !have[name] || slices.Contains(want.Tags, name) {
				continue
			}
			if err := b.tags.Remove(ctx, from, name, tags.Options{Origin: origin}); err != nil {
				fail(from, fmt.Errorf("untag %s: %w", name, err))
				continue
			}
			r.Untagged++
		}

		linked := map[metaLink]bool{}
		if ls, err := b.links.List(ctx, from); err == nil {
			for _, l := range ls {
				linked[metaLink{To: l.Value.To, Label: l.Value.Label}] = true
			}
		}
		for _, l := range want.Links {
			to := metaLink{To: store(l.To), Label: l.Label}
			if linked[to] || (base != nil && slices.Contains(was.Links, l)) {
				continue
			}
			if _, err := b.links.Add(ctx, from, to.To, links.Options{Origin: origin, Label: l.Label}); err != nil {
				fail(from, fmt.Errorf("link to %s: %w", to.To, err))
				continue
			}
			r.Linked++
		}
		for _, l := range was.Links {
			to := metaLink{To: store(l.To), Label: l.Label}
			if !linked[to] || slices.Contains(want.Links, l) {
				continue
			}
			if err := b.links.Remove(ctx, from, to.To, links.Options{Origin: origin, Label: l.Label}); err != nil {
				fail(from, fmt.Errorf("unlink %s: %w", to.To, err))
				continue
			}
			r.Unlinked++
		}
	}
	return r, failed
}
