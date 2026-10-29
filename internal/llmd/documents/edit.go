package documents

import (
	"context"
	"errors"
	"strings"

	"github.com/jpl-au/llmd/internal/errs"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// ErrNoMatch indicates that the search string was not found anywhere in
// the document. The agent's mental model of the file is stale; it should
// re-read before attempting another edit.
var ErrNoMatch = errors.New("the old text is not in the document.")

// ErrNotUnique indicates that the search string matched more than one
// place in the document. The agent must either expand the search string
// with surrounding context to disambiguate, or set ReplaceAll to
// substitute every occurrence at once.
var ErrNotUnique = errors.New("the old text appears more than once.")

// ErrNoOp indicates that old and new are identical, so the edit would
// produce no change. Rejecting these catches confused agents early
// rather than burning a version on a no-op write.
var ErrNoOp = errors.New("the old and new text are the same.")

// Edit performs a search/replace on a document and writes the result as
// a new version.
//
// When ReplaceAll is false (the default), the search string must occur
// exactly once in the document. Zero matches returns ErrNoMatch; more
// than one returns ErrNotUnique with the count attached. When
// ReplaceAll is true, every occurrence is substituted and only the
// zero-match case errors.
//
// In all modes, an old string equal to new returns ErrNoOp without
// touching storage.
func (d *Documents) Edit(ctx context.Context, path, old, new string, opts EditOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if old == new {
		return nil, errs.New(ErrNoOp, "the old and new text are the same.")
	}

	doc, err := d.Read(ctx, path)
	if err != nil {
		return nil, err
	}

	count := strings.Count(doc.Content, old)
	if count == 0 {
		return nil, errs.New(ErrNoMatch, "the old text is not in %s. Cat it and copy the text exactly.", path)
	}

	var result string
	if opts.ReplaceAll {
		result = strings.ReplaceAll(doc.Content, old, new)
	} else {
		if count > 1 {
			return nil, errs.New(ErrNotUnique, "the old text appears %d times in %s.", count, path)
		}
		idx := strings.Index(doc.Content, old)
		var b strings.Builder
		b.Grow(len(doc.Content) + len(new) - len(old))
		b.WriteString(doc.Content[:idx])
		b.WriteString(new)
		b.WriteString(doc.Content[idx+len(old):])
		result = b.String()
	}

	return d.Write(ctx, path, result, WriteOptions{Origin: opts.Origin})
}
