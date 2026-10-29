package host

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// sortLinks orders links by (From, To) so lists compare independently
// of insertion order.
func sortLinks(links []sdk.Link) {
	slices.SortFunc(links, func(a, b sdk.Link) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
}

// TestLinksAdd adds links in order; every add but the last must
// succeed and the last is compared against wantErr. The outgoing
// links of path are then compared against want.
func TestLinksAdd(t *testing.T) {
	tests := []struct {
		name    string
		docs    []string
		adds    []sdk.Link
		wantErr error
		path    string
		want    []sdk.Link
	}{
		{
			name: "with label",
			docs: []string{"a", "b"},
			adds: []sdk.Link{{From: "a", To: "b", Label: "related"}},
			path: "a",
			want: []sdk.Link{{From: "a", To: "b", Label: "related"}},
		},
		{
			name: "without label",
			docs: []string{"x", "y"},
			adds: []sdk.Link{{From: "x", To: "y"}},
			path: "x",
			want: []sdk.Link{{From: "x", To: "y"}},
		},
		{
			name: "multiple labels",
			docs: []string{"a", "b", "c"},
			adds: []sdk.Link{{From: "a", To: "b", Label: "blocks"}, {From: "a", To: "c", Label: "relates"}},
			path: "a",
			want: []sdk.Link{{From: "a", To: "b", Label: "blocks"}, {From: "a", To: "c", Label: "relates"}},
		},
		{
			// Adding the same link again must not create a duplicate.
			name:    "duplicate",
			docs:    []string{"a", "b"},
			adds:    []sdk.Link{{From: "a", To: "b", Label: "related"}, {From: "a", To: "b", Label: "related"}},
			wantErr: sdk.ErrExists,
			path:    "a",
			want:    []sdk.Link{{From: "a", To: "b", Label: "related"}},
		},
		{
			name:    "self link",
			docs:    []string{"doc"},
			adds:    []sdk.Link{{From: "doc", To: "doc"}},
			wantErr: sdk.ErrInvalidArg,
			path:    "doc",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			for _, path := range tt.docs {
				writeDoc(t, c, path, "x")
			}
			var err error
			for i, l := range tt.adds {
				err = c.Links.Add(l.From, l.To, l.Label, "alice")
				if err != nil && i < len(tt.adds)-1 {
					t.Fatalf("Add(%q, %q, %q) error = %v", l.From, l.To, l.Label, err)
				}
			}
			last := tt.adds[len(tt.adds)-1]
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Add(%q, %q, %q) error = %v, want %v", last.From, last.To, last.Label, err, tt.wantErr)
			}
			got, err := c.Links.List(tt.path, "out")
			if err != nil {
				t.Fatalf("List(%q, out) error = %v", tt.path, err)
			}
			sortLinks(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%q, out) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLinksRemove(t *testing.T) {
	tests := []struct {
		name    string
		linked  bool // add a->b before removing
		wantErr error
	}{
		{"existing", true, nil},
		{"nonexistent", false, sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			writeDoc(t, c, "a", "x")
			writeDoc(t, c, "b", "x")
			if tt.linked {
				if err := c.Links.Add("a", "b", "", "alice"); err != nil {
					t.Fatalf("Add(a, b) error = %v", err)
				}
			}
			err := c.Links.Remove("a", "b", "alice")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Remove(a, b) error = %v, want %v", err, tt.wantErr)
			}
			got, err := c.Links.List("a", "out")
			if err != nil {
				t.Fatalf("List(a, out) error = %v", err)
			}
			if len(got) != 0 {
				t.Errorf("List(a, out) after Remove = %v, want none", got)
			}
		})
	}
}

func TestLinksList(t *testing.T) {
	tests := []struct {
		name  string
		docs  []string
		links []sdk.Link
		path  string
		dir   string
		want  []sdk.Link
	}{
		{
			name:  "outgoing",
			docs:  []string{"hub", "out1", "out2"},
			links: []sdk.Link{{From: "hub", To: "out1"}, {From: "hub", To: "out2"}},
			path:  "hub",
			dir:   "out",
			want:  []sdk.Link{{From: "hub", To: "out1"}, {From: "hub", To: "out2"}},
		},
		{
			name:  "incoming",
			docs:  []string{"target", "src1", "src2"},
			links: []sdk.Link{{From: "src1", To: "target"}, {From: "src2", To: "target"}},
			path:  "target",
			dir:   "in",
			want:  []sdk.Link{{From: "src1", To: "target"}, {From: "src2", To: "target"}},
		},
		{
			name:  "both",
			docs:  []string{"a", "b", "c"},
			links: []sdk.Link{{From: "a", To: "b"}, {From: "c", To: "b"}},
			path:  "b",
			dir:   "both",
			want:  []sdk.Link{{From: "a", To: "b"}, {From: "c", To: "b"}},
		},
		{
			name: "empty",
			docs: []string{"lonely"},
			path: "lonely",
			dir:  "out",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			for _, path := range tt.docs {
				writeDoc(t, c, path, "x")
			}
			for _, l := range tt.links {
				if err := c.Links.Add(l.From, l.To, l.Label, "alice"); err != nil {
					t.Fatalf("Add(%q, %q) error = %v", l.From, l.To, err)
				}
			}
			got, err := c.Links.List(tt.path, tt.dir)
			if err != nil {
				t.Fatalf("List(%q, %q) error = %v", tt.path, tt.dir, err)
			}
			sortLinks(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}
