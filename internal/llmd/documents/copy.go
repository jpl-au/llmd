package documents

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// Copy duplicates a document from src to dst.
// Creates a new version 1 at dst with the content from src.
func (d *Documents) Copy(ctx context.Context, src, dst string, opts CopyOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Check destination doesn't exist
	if ok, err := d.Exists(ctx, dst); err != nil {
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("destination already exists: %s", dst)
	}

	// Read source
	doc, err := d.Read(ctx, src)
	if err != nil {
		return nil, err
	}

	// Write to destination
	return d.Write(ctx, dst, doc.Content, WriteOptions(opts))
}
