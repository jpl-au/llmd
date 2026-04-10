package host

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func TestTagsAdd(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := sdk.Tags.Add("doc", "important", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	tags, err := sdk.Tags.List("doc")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(tags))
	}
	if tags[0].Name != "important" {
		t.Errorf("name = %q, want %q", tags[0].Name, "important")
	}
}

func TestTagsAddMultiple(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sdk.Tags.Add("doc", "feature", "alice"); err != nil {
		t.Fatalf("Add feature: %v", err)
	}
	if err := sdk.Tags.Add("doc", "urgent", "alice"); err != nil {
		t.Fatalf("Add urgent: %v", err)
	}

	tags, _ := sdk.Tags.List("doc")
	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}
}

func TestTagsRemove(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sdk.Tags.Add("doc", "temp", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sdk.Tags.Remove("doc", "temp", "alice"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	tags, _ := sdk.Tags.List("doc")
	if len(tags) != 0 {
		t.Errorf("got %d tags after Remove, want 0", len(tags))
	}
}

func TestTagsList(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	tags, err := sdk.Tags.List("doc")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("got %d tags on untagged doc, want 0", len(tags))
	}
}

func TestTagsAll(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Tags.Add("a", "feature", "alice"); err != nil {
		t.Fatalf("Add a/feature: %v", err)
	}
	if err := sdk.Tags.Add("b", "feature", "alice"); err != nil {
		t.Fatalf("Add b/feature: %v", err)
	}
	if err := sdk.Tags.Add("a", "bug", "alice"); err != nil {
		t.Fatalf("Add a/bug: %v", err)
	}

	infos, err := sdk.Tags.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d tag infos, want 2", len(infos))
	}

	// Find the "feature" tag and check count
	for _, info := range infos {
		if info.Name == "feature" && info.Count != 2 {
			t.Errorf("feature count = %d, want 2", info.Count)
		}
		if info.Name == "bug" && info.Count != 1 {
			t.Errorf("bug count = %d, want 1", info.Count)
		}
	}
}

func TestTagsAllEmpty(t *testing.T) {
	testHost(t)

	infos, err := sdk.Tags.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("got %d infos, want 0", len(infos))
	}
}

func TestTagsFind(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Documents.Write("c", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write c: %v", err)
	}
	if err := sdk.Tags.Add("a", "release", "alice"); err != nil {
		t.Fatalf("Tag a: %v", err)
	}
	if err := sdk.Tags.Add("c", "release", "alice"); err != nil {
		t.Fatalf("Tag c: %v", err)
	}

	paths, err := sdk.Tags.Find("release")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("got %d paths, want 2", len(paths))
	}
}

func TestTagsFindEmpty(t *testing.T) {
	testHost(t)

	paths, err := sdk.Tags.Find("nonexistent")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("got %d paths, want 0", len(paths))
	}
}

func TestTagsAddDuplicate(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := sdk.Tags.Add("doc", "dupe", "alice"); err != nil {
		t.Fatalf("Tag dupe: %v", err)
	}
	_ = sdk.Tags.Add("doc", "dupe", "alice") // duplicate - expected to fail

	tags, _ := sdk.Tags.List("doc")
	if len(tags) != 1 {
		t.Errorf("duplicate add: got %d tags, want 1", len(tags))
	}
}

func TestTagsRemoveNonexistent(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	err := sdk.Tags.Remove("doc", "nope", "alice")
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Remove error = %v, want sdk.ErrNotFound", err)
	}
}

func TestTagsListMultiple(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sdk.Tags.Add("doc", "alpha", "alice"); err != nil {
		t.Fatalf("Add alpha: %v", err)
	}
	if err := sdk.Tags.Add("doc", "beta", "alice"); err != nil {
		t.Fatalf("Add beta: %v", err)
	}
	if err := sdk.Tags.Add("doc", "gamma", "alice"); err != nil {
		t.Fatalf("Add gamma: %v", err)
	}

	tags, err := sdk.Tags.List("doc")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("got %d tags, want 3", len(tags))
	}

	names := map[string]bool{}
	for _, tag := range tags {
		names[tag.Name] = true
	}
	if !names["alpha"] || !names["beta"] || !names["gamma"] {
		t.Errorf("missing expected tags in %v", names)
	}
}

func TestTagsFindMultipleDocs(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Documents.Write("c", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write c: %v", err)
	}

	if err := sdk.Tags.Add("a", "shared", "alice"); err != nil {
		t.Fatalf("Add shared a: %v", err)
	}
	if err := sdk.Tags.Add("b", "shared", "alice"); err != nil {
		t.Fatalf("Add shared b: %v", err)
	}
	if err := sdk.Tags.Add("c", "unique", "alice"); err != nil {
		t.Fatalf("Add unique: %v", err)
	}

	paths, _ := sdk.Tags.Find("shared")
	if len(paths) != 2 {
		t.Errorf("Find shared: got %d, want 2", len(paths))
	}

	paths, _ = sdk.Tags.Find("unique")
	if len(paths) != 1 {
		t.Errorf("Find unique: got %d, want 1", len(paths))
	}
}
