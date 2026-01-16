// edit.go implements in-place document modification operations.
//
// Separated from write.go because edit operations read-then-write with
// transformation logic between. This file handles search/replace and
// line-range edits that modify existing content rather than replacing it.
//
// Design: Edits create new versions, preserving the original. The edit
// package handles the actual text transformation; this file orchestrates
// the read-transform-write cycle and filesystem sync.

package document

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/edit"
	"github.com/jpl-au/llmd/internal/store"
)

// Edit performs a search/replace edit on a document.
// p can be a document path or a key.
func (s *Service) Edit(ctx context.Context, p string, opts edit.Options) error {
	doc, err := s.Resolve(ctx, p, false)
	if err != nil {
		return fmt.Errorf("edit %q: %w", p, err)
	}
	p = doc.Path // Use resolved path

	content, err := edit.Replace(doc.Content, opts.Old, opts.New, opts.CaseInsensitive)
	if err != nil {
		return fmt.Errorf("edit %q: %w", p, err)
	}

	author := opts.Author
	if author == "" {
		author = DefaultAuthor
	}

	writeOpts := store.WriteOptions{
		Author:     author,
		Message:    opts.Message,
		MaxPath:    s.maxPath,
		MaxContent: s.maxContent,
	}

	if err := s.store.Write(ctx, p, content, writeOpts); err != nil {
		return fmt.Errorf("edit %q: write: %w", p, err)
	}

	if err := s.syncWrite(p, content); err != nil {
		return fmt.Errorf("sync %q: %w", p, err)
	}
	return nil
}

// EditLineRange replaces a range of lines in a document.
// p can be a document path or a key.
func (s *Service) EditLineRange(ctx context.Context, p string, opts edit.LineRangeOptions, replacement string) error {
	doc, err := s.Resolve(ctx, p, false)
	if err != nil {
		return fmt.Errorf("edit lines %q: %w", p, err)
	}
	p = doc.Path // Use resolved path

	content, err := edit.ReplaceLines(doc.Content, opts.Start, opts.End, replacement)
	if err != nil {
		return fmt.Errorf("edit lines %q: %w", p, err)
	}

	author := opts.Author
	if author == "" {
		author = DefaultAuthor
	}

	writeOpts := store.WriteOptions{
		Author:     author,
		Message:    opts.Message,
		MaxPath:    s.maxPath,
		MaxContent: s.maxContent,
	}

	if err := s.store.Write(ctx, p, content, writeOpts); err != nil {
		return fmt.Errorf("edit lines %q: write: %w", p, err)
	}

	if err := s.syncWrite(p, content); err != nil {
		return fmt.Errorf("sync %q: %w", p, err)
	}
	return nil
}
