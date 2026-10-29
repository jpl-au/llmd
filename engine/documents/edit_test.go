package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine/documents"
)

func TestEdit(t *testing.T) {
	tests := []struct {
		name       string
		content    string // initial content of docs/readme
		old        string
		new        string
		replaceAll bool
		want       string // content after a successful edit
		wantErr    error
	}{
		{
			name:    "replaces unique match",
			content: "hello world",
			old:     "world",
			new:     "universe",
			want:    "hello universe",
		},
		{
			name:       "replace all",
			content:    "foo bar foo baz foo",
			old:        "foo",
			new:        "qux",
			replaceAll: true,
			want:       "qux bar qux baz qux",
		},
		{
			// The documented escape hatch: when the bare search string is
			// ambiguous, the agent expands it with surrounding context
			// until exactly one match remains.
			name:    "disambiguated by context",
			content: "foo bar foo baz",
			old:     "foo baz",
			new:     "qux baz",
			want:    "foo bar qux baz",
		},
		{
			name:    "no match",
			content: "hello world",
			old:     "notfound",
			new:     "replacement",
			wantErr: documents.ErrNoMatch,
		},
		{
			name:    "not unique",
			content: "foo bar foo",
			old:     "foo",
			new:     "qux",
			wantErr: documents.ErrNotUnique,
		},
		{
			name:    "no-op",
			content: "hello",
			old:     "hello",
			new:     "hello",
			wantErr: documents.ErrNoOp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			mustWrite(t, s, "docs/readme", tt.content)

			opts := testEditOpts()
			opts.ReplaceAll = tt.replaceAll
			doc, err := s.Documents.Edit(ctx, "docs/readme", tt.old, tt.new, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Edit(%q, %q) error = %v, want %v", tt.old, tt.new, err, tt.wantErr)
			}
			if err != nil {
				// A failed edit must not touch the document: no new
				// version may be created.
				got, err := s.Documents.Read(ctx, "docs/readme")
				if err != nil {
					t.Fatalf("Read() after failed Edit error = %v", err)
				}
				if got.Version != 1 {
					t.Errorf("Read() after failed Edit: Version = %d, want 1", got.Version)
				}
				if got.Content != tt.content {
					t.Errorf("Read() after failed Edit: Content = %q, want %q", got.Content, tt.content)
				}
				return
			}

			if doc.Content != tt.want {
				t.Errorf("Edit(%q, %q).Content = %q, want %q", tt.old, tt.new, doc.Content, tt.want)
			}
			if doc.Version != 2 {
				t.Errorf("Edit(%q, %q).Version = %d, want 2", tt.old, tt.new, doc.Version)
			}
		})
	}
}
