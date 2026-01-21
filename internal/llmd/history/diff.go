package history

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// DiffResult contains the result of comparing two documents.
type DiffResult struct {
	A *document.Document
	B *document.Document
}

// Diff compares two documents.
// a and b can be filesystem paths, llmd paths, or 9-char keys.
func (h *History) Diff(ctx context.Context, a, b string, opts ...DiffOptions) (*DiffResult, error) {
	var opt DiffOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	_ = opt // TODO: use for unified diff formatting

	docA, err := h.docs.Resolve(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("resolving a: %w", err)
	}

	docB, err := h.docs.Resolve(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("resolving b: %w", err)
	}

	return &DiffResult{
		A: docA,
		B: docB,
	}, nil
}
