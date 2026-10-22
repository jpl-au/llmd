package host

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// tagNames returns the sorted tag names on a document.
func tagNames(t *testing.T, path string) []string {
	t.Helper()
	tags, err := sdk.Tags.List(path)
	if err != nil {
		t.Fatalf("List(%q) error = %v", path, err)
	}
	var names []string
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	slices.Sort(names)
	return names
}

// TestTagsAdd adds tags to "doc" in order; every add but the last must
// succeed and the last is compared against wantErr.
func TestTagsAdd(t *testing.T) {
	tests := []struct {
		name    string
		adds    []string
		wantErr error
		want    []string // sorted tag names on doc
	}{
		{"single", []string{"important"}, nil, []string{"important"}},
		{"multiple", []string{"feature", "urgent"}, nil, []string{"feature", "urgent"}},
		{"duplicate", []string{"dupe", "dupe"}, sdk.ErrExists, []string{"dupe"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			writeDoc(t, "doc", "x")
			var err error
			for i, name := range tt.adds {
				err = sdk.Tags.Add("doc", name, "alice")
				if err != nil && i < len(tt.adds)-1 {
					t.Fatalf("Add(doc, %q) error = %v", name, err)
				}
			}
			last := tt.adds[len(tt.adds)-1]
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Add(doc, %q) error = %v, want %v", last, err, tt.wantErr)
			}
			if got := tagNames(t, "doc"); !slices.Equal(got, tt.want) {
				t.Errorf("List(doc) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsRemove(t *testing.T) {
	tests := []struct {
		name    string
		tagged  bool // add the tag before removing
		wantErr error
	}{
		{"existing", true, nil},
		{"nonexistent", false, sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			writeDoc(t, "doc", "x")
			if tt.tagged {
				if err := sdk.Tags.Add("doc", "temp", "alice"); err != nil {
					t.Fatalf("Add(doc, temp) error = %v", err)
				}
			}
			err := sdk.Tags.Remove("doc", "temp", "alice")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Remove(doc, temp) error = %v, want %v", err, tt.wantErr)
			}
			if got := tagNames(t, "doc"); len(got) != 0 {
				t.Errorf("List(doc) after Remove = %v, want none", got)
			}
		})
	}
}

func TestTagsList(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{"untagged", nil, nil},
		{"multiple", []string{"alpha", "beta", "gamma"}, []string{"alpha", "beta", "gamma"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			writeDoc(t, "doc", "x")
			for _, name := range tt.tags {
				if err := sdk.Tags.Add("doc", name, "alice"); err != nil {
					t.Fatalf("Add(doc, %q) error = %v", name, err)
				}
			}
			if got := tagNames(t, "doc"); !slices.Equal(got, tt.want) {
				t.Errorf("List(doc) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsAll(t *testing.T) {
	tests := []struct {
		name string
		tags []sdk.Tag // Path, Name pairs to add
		want []sdk.TagInfo
	}{
		{
			name: "counts per tag",
			tags: []sdk.Tag{{Path: "a", Name: "feature"}, {Path: "b", Name: "feature"}, {Path: "a", Name: "bug"}},
			want: []sdk.TagInfo{{Name: "bug", Count: 1}, {Name: "feature", Count: 2}},
		},
		{
			name: "empty store",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, tag := range tt.tags {
				if ok, _ := sdk.Documents.Exists(tag.Path); !ok {
					writeDoc(t, tag.Path, "x")
				}
				if err := sdk.Tags.Add(tag.Path, tag.Name, "alice"); err != nil {
					t.Fatalf("Add(%q, %q) error = %v", tag.Path, tag.Name, err)
				}
			}
			got, err := sdk.Tags.All()
			if err != nil {
				t.Fatalf("All() error = %v", err)
			}
			slices.SortFunc(got, func(a, b sdk.TagInfo) int { return cmp.Compare(a.Name, b.Name) })
			if !slices.Equal(got, tt.want) {
				t.Errorf("All() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsFind(t *testing.T) {
	shared := []sdk.Tag{{Path: "a", Name: "shared"}, {Path: "b", Name: "shared"}, {Path: "c", Name: "unique"}}
	tests := []struct {
		name  string
		tags  []sdk.Tag // Path, Name pairs to add
		query string
		want  []string // sorted paths
	}{
		{
			name:  "two documents",
			tags:  []sdk.Tag{{Path: "a", Name: "release"}, {Path: "c", Name: "release"}},
			query: "release",
			want:  []string{"a", "c"},
		},
		{"shared tag", shared, "shared", []string{"a", "b"}},
		{"unique tag", shared, "unique", []string{"c"}},
		{"no documents", nil, "nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, tag := range tt.tags {
				if ok, _ := sdk.Documents.Exists(tag.Path); !ok {
					writeDoc(t, tag.Path, "x")
				}
				if err := sdk.Tags.Add(tag.Path, tag.Name, "alice"); err != nil {
					t.Fatalf("Add(%q, %q) error = %v", tag.Path, tag.Name, err)
				}
			}
			got, err := sdk.Tags.Find(tt.query)
			if err != nil {
				t.Fatalf("Find(%q) error = %v", tt.query, err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Find(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
