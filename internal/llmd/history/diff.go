package history

import (
	"context"
	"fmt"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// DiffResult contains the result of comparing two documents.
type DiffResult struct {
	A       *document.Document
	B       *document.Document
	Unified string // Unified diff output
	Stats   DiffStats
}

// DiffStats contains statistics about the diff.
type DiffStats struct {
	Added   int
	Removed int
}

// Diff compares two documents.
// a and b can be filesystem paths, llmd paths, llmd path:version, or 9-char keys.
func (h *History) Diff(ctx context.Context, a, b string, opts ...DiffOptions) (*DiffResult, error) {
	var opt DiffOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	docA, err := h.readIdentifier(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", a, err)
	}

	docB, err := h.readIdentifier(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", b, err)
	}

	// Compute unified diff
	context := opt.Context
	if context == 0 {
		context = 3 // default context lines
	}

	labelA := formatLabel(docA)
	labelB := formatLabel(docB)

	edits := udiff.Strings(docA.Content, docB.Content)
	unified, err := udiff.ToUnified(labelA, labelB, docA.Content, edits, context)
	if err != nil {
		return nil, fmt.Errorf("computing diff: %w", err)
	}

	// Compute stats
	stats := computeStats(edits, docA.Content)

	return &DiffResult{
		A:       docA,
		B:       docB,
		Unified: unified,
		Stats:   stats,
	}, nil
}

// formatLabel creates a label for diff output.
func formatLabel(doc *document.Document) string {
	if doc.Version > 0 {
		return fmt.Sprintf("%s:%d", doc.Path, doc.Version)
	}
	return doc.Path
}

// computeStats calculates lines added and removed.
func computeStats(edits []udiff.Edit, original string) DiffStats {
	var stats DiffStats
	for _, e := range edits {
		// Count removed lines (from original content being replaced)
		removed := strings.Count(original[e.Start:e.End], "\n")
		if e.End > e.Start && original[e.End-1] != '\n' {
			removed++ // partial line counts as a line
		}
		stats.Removed += removed

		// Count added lines (new content)
		added := strings.Count(e.New, "\n")
		if len(e.New) > 0 && e.New[len(e.New)-1] != '\n' {
			added++ // partial line counts as a line
		}
		stats.Added += added
	}
	return stats
}

// readIdentifier resolves an identifier (path, key, or either with
// :version suffix) and reads the document from the store.
func (h *History) readIdentifier(ctx context.Context, value string) (*document.Document, error) {
	path, version, _ := resolve.Identifier(ctx, value, h.docs.KeyToPath)
	var opts documents.ReadOptions
	if version != nil {
		opts.Version = version
	}
	return h.docs.Read(ctx, path, opts)
}
