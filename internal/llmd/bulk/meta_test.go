package bulk_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

// metaStore returns a store holding docs, each tagged and linked as
// given: tags map a path to its tag names, links a path to the paths
// it links to.
func metaStore(t *testing.T, docs []string, tagged map[string][]string, linked map[string][]string) *llmd.Store {
	t.Helper()
	s := testStore(t)
	ctx := t.Context()
	for _, p := range docs {
		if _, err := s.Documents.Write(ctx, p, "# "+p, testWriteOpts()); err != nil {
			t.Fatalf("Write(%q) error = %v", p, err)
		}
	}
	for p, names := range tagged {
		for _, n := range names {
			if _, err := s.Tags.Add(ctx, p, n, tags.Options{Origin: testWriteOpts().Origin}); err != nil {
				t.Fatalf("Tags.Add(%q, %q) error = %v", p, n, err)
			}
		}
	}
	for p, tos := range linked {
		for _, to := range tos {
			if _, err := s.Links.Add(ctx, p, to, links.Options{Origin: testWriteOpts().Origin}); err != nil {
				t.Fatalf("Links.Add(%q, %q) error = %v", p, to, err)
			}
		}
	}
	return s
}

// tagsOf returns a document's tag names, sorted.
func tagsOf(t *testing.T, s *llmd.Store, path string) []string {
	t.Helper()
	ts, err := s.Tags.List(t.Context(), path)
	if err != nil {
		t.Fatalf("Tags.List(%q) error = %v", path, err)
	}
	var names []string
	for _, tg := range ts {
		names = append(names, tg.Value.Tag)
	}
	slices.Sort(names)
	return names
}

// linksOf returns the paths a document links to, sorted.
func linksOf(t *testing.T, s *llmd.Store, path string) []string {
	t.Helper()
	ls, err := s.Links.List(t.Context(), path)
	if err != nil {
		t.Fatalf("Links.List(%q) error = %v", path, err)
	}
	var tos []string
	for _, l := range ls {
		tos = append(tos, l.Value.To)
	}
	slices.Sort(tos)
	return tos
}

// TestMetaExportImport checks that a prefix export writes tags and
// links relative to the prefix, leaving out links that leave it, and
// that importing the directory under another prefix restores them.
func TestMetaExportImport(t *testing.T) {
	src := metaStore(t,
		[]string{"docs/api", "docs/auth", "notes/todo"},
		map[string][]string{"docs/api": {"backend", "reviewed"}, "docs/auth": {"backend"}},
		map[string][]string{"docs/api": {"docs/auth", "notes/todo"}},
	)
	dir := t.TempDir()
	if _, err := src.Bulk.Export(t.Context(), "docs/", dir, bulk.ExportOptions{}); err != nil {
		t.Fatalf("Export(docs/) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".meta.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(.meta.yaml) error = %v", err)
	}
	want := "api:\n    tags:\n        - backend\n        - reviewed\n    links:\n        - to: auth\nauth:\n    tags:\n        - backend\n"
	if string(data) != want {
		t.Errorf(".meta.yaml = %q, want %q", data, want)
	}

	dst := testStore(t)
	r, err := dst.Bulk.Import(t.Context(), dir, bulk.ImportOptions{Origin: testWriteOpts().Origin, Prefix: "copy"})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if r.Meta != (bulk.MetaResult{Tagged: 3, Linked: 1}) || len(r.Errors) != 0 {
		t.Errorf("Import().Meta = %+v, errors %v, want 3 tagged, 1 linked, no errors", r.Meta, r.Errors)
	}
	if got := tagsOf(t, dst, "copy/api"); !slices.Equal(got, []string{"backend", "reviewed"}) {
		t.Errorf("tags of copy/api = %v, want [backend reviewed]", got)
	}
	if got := linksOf(t, dst, "copy/api"); !slices.Equal(got, []string{"copy/auth"}) {
		t.Errorf("links of copy/api = %v, want [copy/auth]", got)
	}

	// A second import adds nothing.
	again, err := dst.Bulk.Import(t.Context(), dir, bulk.ImportOptions{Origin: testWriteOpts().Origin, Prefix: "copy"})
	if err != nil || again.Meta != (bulk.MetaResult{}) {
		t.Errorf("second Import().Meta = %+v, %v, want no changes", again.Meta, err)
	}
}

// TestMetaMirrorPush covers the push's three-way rule: what changed in
// .meta.yaml since the last pull is applied, and tags or links made in
// the store since are kept.
func TestMetaMirrorPush(t *testing.T) {
	s := metaStore(t,
		[]string{"api", "auth", "errors"},
		map[string][]string{"api": {"backend", "old"}},
		map[string][]string{"api": {"auth"}},
	)
	ctx := t.Context()
	opts := bulk.MirrorPushOptions{Origin: testWriteOpts().Origin}
	dir := filepath.Join(t.TempDir(), "llmd")
	if _, err := s.Bulk.Mirror(ctx, "", dir); err != nil {
		t.Fatalf("Mirror() error = %v", err)
	}

	// Someone else, through git: drops the "old" tag and the link to
	// auth, tags auth, and links api to errors.
	edited := "api:\n  tags: [backend]\n  links:\n    - to: errors\nauth:\n  tags: [security]\n"
	if err := os.WriteFile(filepath.Join(dir, ".meta.yaml"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// Meanwhile, here: a tag made in the store after the pull.
	if _, err := s.Tags.Add(ctx, "errors", "local", tags.Options{Origin: testWriteOpts().Origin}); err != nil {
		t.Fatal(err)
	}

	r, err := s.Bulk.MirrorPush(ctx, dir, opts)
	if err != nil {
		t.Fatalf("MirrorPush() error = %v", err)
	}
	if want := (bulk.MetaResult{Tagged: 1, Untagged: 1, Linked: 1, Unlinked: 1}); r.Meta != want || len(r.Errors) != 0 {
		t.Errorf("MirrorPush().Meta = %+v, errors %v, want %+v", r.Meta, r.Errors, want)
	}
	for _, c := range []struct {
		path      string
		tags, tos []string
	}{
		{"api", []string{"backend"}, []string{"errors"}},
		{"auth", []string{"security"}, nil},
		{"errors", []string{"local"}, nil},
	} {
		if got := tagsOf(t, s, c.path); !slices.Equal(got, c.tags) {
			t.Errorf("tags of %s = %v, want %v", c.path, got, c.tags)
		}
		if got := linksOf(t, s, c.path); !slices.Equal(got, c.tos) {
			t.Errorf("links of %s = %v, want %v", c.path, got, c.tos)
		}
	}

	// The push became the base: pushing again changes nothing, and a
	// pull writes the store's tags back, the local one included.
	if again, err := s.Bulk.MirrorPush(ctx, dir, opts); err != nil || again.Meta != (bulk.MetaResult{}) {
		t.Errorf("second MirrorPush().Meta = %+v, %v, want no changes", again.Meta, err)
	}
	if _, err := s.Bulk.Mirror(ctx, "", dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".meta.yaml"))
	if !strings.Contains(string(data), "errors:\n    tags:\n        - local") {
		t.Errorf(".meta.yaml after pull = %q, want the local tag on errors", data)
	}
}

// TestMetaFreshClone checks a push with no manifest, as in a fresh
// clone: the documents and then their tags and links are created.
func TestMetaFreshClone(t *testing.T) {
	s := testStore(t)
	dir := filepath.Join(t.TempDir(), "llmd")
	for name, content := range map[string]string{
		"api.md":     "# api",
		"auth.md":    "# auth",
		".meta.yaml": "api:\n  tags: [backend]\n  links:\n    - to: auth\n      label: depends-on\n    - to: missing\n",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Bulk.MirrorPush(t.Context(), dir, bulk.MirrorPushOptions{Origin: testWriteOpts().Origin})
	if err != nil {
		t.Fatalf("MirrorPush() error = %v", err)
	}
	if len(r.Created) != 2 || r.Meta != (bulk.MetaResult{Tagged: 1, Linked: 1}) {
		t.Errorf("MirrorPush() = created %v, meta %+v, want 2 created, 1 tagged, 1 linked", r.Created, r.Meta)
	}
	// The link to a document that does not exist is reported, not fatal.
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0].Err.Error(), "missing") {
		t.Errorf("MirrorPush().Errors = %v, want one for the link to missing", r.Errors)
	}
}
