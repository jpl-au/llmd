package documents

import (
	"context"
	"errors"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/document"
)

var ErrNoMatch = errors.New("no match found")

// Edit performs a search/replace on a document.
// Returns ErrNoMatch if old string is not found.
func (d *Documents) Edit(ctx context.Context, path, old, new string, opts EditOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	doc, err := d.Read(ctx, path)
	if err != nil {
		return nil, err
	}

	var result string
	if opts.ReplaceAll {
		if !strings.Contains(doc.Content, old) {
			return nil, ErrNoMatch
		}
		result = strings.ReplaceAll(doc.Content, old, new)
	} else {
		idx := strings.Index(doc.Content, old)
		if idx == -1 {
			return nil, ErrNoMatch
		}

		var b strings.Builder
		b.Grow(len(doc.Content) + len(new) - len(old))
		b.WriteString(doc.Content[:idx])
		b.WriteString(new)
		b.WriteString(doc.Content[idx+len(old):])
		result = b.String()
	}

	return d.Write(ctx, path, result, WriteOptions{WriteContext: opts.WriteContext})
}
