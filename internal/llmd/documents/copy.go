package documents

import (
	"context"

	"github.com/jpl-au/llmd/internal/errs"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// Copy duplicates a document from src to dst by writing src's content
// to dst. A free dst gets version 1; a deleted document at dst is
// continued, as any write to it is. A live dst is refused.
func (d *Documents) Copy(ctx context.Context, src, dst string, opts CopyOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Check destination doesn't exist
	ok, err := d.Exists(ctx, dst)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, errs.New(ErrExists, "%s already exists.", dst)
	}

	// Read source
	doc, err := d.Read(ctx, src)
	if err != nil {
		return nil, err
	}

	// Write to destination
	return d.Write(ctx, dst, doc.Content, WriteOptions(opts))
}
