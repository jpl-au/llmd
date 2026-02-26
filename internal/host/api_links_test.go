package host

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func TestLinksAdd(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}

	if err := sdk.Links.Add("a", "b", "related", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	links, err := sdk.Links.List("a", "out")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if links[0].To != "b" {
		t.Errorf("link to = %q, want %q", links[0].To, "b")
	}
	if links[0].Label != "related" {
		t.Errorf("label = %q, want %q", links[0].Label, "related")
	}
}

func TestLinksAddNoLabel(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("x", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write x: %v", err)
	}
	if err := sdk.Documents.Write("y", []byte("y"), "alice", ""); err != nil {
		t.Fatalf("Write y: %v", err)
	}

	if err := sdk.Links.Add("x", "y", "", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	links, _ := sdk.Links.List("x", "out")
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if links[0].Label != "" {
		t.Errorf("label = %q, want empty", links[0].Label)
	}
}

func TestLinksRemove(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Links.Add("a", "b", "", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sdk.Links.Remove("a", "b", "alice"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	links, _ := sdk.Links.List("a", "out")
	if len(links) != 0 {
		t.Errorf("got %d links after Remove, want 0", len(links))
	}
}

func TestLinksListOutgoing(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("center", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write center: %v", err)
	}
	if err := sdk.Documents.Write("out1", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write out1: %v", err)
	}
	if err := sdk.Documents.Write("out2", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write out2: %v", err)
	}

	if err := sdk.Links.Add("center", "out1", "", "alice"); err != nil {
		t.Fatalf("Add center->out1: %v", err)
	}
	if err := sdk.Links.Add("center", "out2", "", "alice"); err != nil {
		t.Fatalf("Add center->out2: %v", err)
	}

	links, _ := sdk.Links.List("center", "out")
	if len(links) != 2 {
		t.Errorf("outgoing: got %d, want 2", len(links))
	}
}

func TestLinksListIncoming(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("target", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write target: %v", err)
	}
	if err := sdk.Documents.Write("src1", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write src1: %v", err)
	}
	if err := sdk.Documents.Write("src2", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write src2: %v", err)
	}

	if err := sdk.Links.Add("src1", "target", "", "alice"); err != nil {
		t.Fatalf("Add src1->target: %v", err)
	}
	if err := sdk.Links.Add("src2", "target", "", "alice"); err != nil {
		t.Fatalf("Add src2->target: %v", err)
	}

	links, _ := sdk.Links.List("target", "in")
	if len(links) != 2 {
		t.Errorf("incoming: got %d, want 2", len(links))
	}
}

func TestLinksListBoth(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Documents.Write("c", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write c: %v", err)
	}

	if err := sdk.Links.Add("a", "b", "", "alice"); err != nil {
		t.Fatalf("Add a->b: %v", err)
	}
	if err := sdk.Links.Add("c", "b", "", "alice"); err != nil {
		t.Fatalf("Add c->b: %v", err)
	}

	links, _ := sdk.Links.List("b", "both")
	if len(links) != 2 {
		t.Errorf("both directions: got %d, want 2", len(links))
	}
}

func TestLinksListEmpty(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("lonely", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	links, err := sdk.Links.List("lonely", "out")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("got %d links, want 0", len(links))
	}
}

func TestLinksAddDuplicate(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}

	if err := sdk.Links.Add("a", "b", "related", "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Adding the same link again should not create a duplicate.
	_ = sdk.Links.Add("a", "b", "related", "alice")

	links, _ := sdk.Links.List("a", "out")
	if len(links) != 1 {
		t.Errorf("duplicate add: got %d links, want 1", len(links))
	}
}

func TestLinksRemoveNonexistent(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}

	err := sdk.Links.Remove("a", "b", "alice")
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Remove error = %v, want sdk.ErrNotFound", err)
	}
}

func TestLinksAddSelfLink(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	err := sdk.Links.Add("doc", "doc", "", "alice")
	if !errors.Is(err, sdk.ErrInvalidArg) {
		t.Errorf("Add self-link error = %v, want sdk.ErrInvalidArg", err)
	}
}

func TestLinksWithLabel(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Documents.Write("c", []byte("x"), "alice", ""); err != nil {
		t.Fatalf("Write c: %v", err)
	}

	if err := sdk.Links.Add("a", "b", "blocks", "alice"); err != nil {
		t.Fatalf("Add blocks: %v", err)
	}
	if err := sdk.Links.Add("a", "c", "relates", "alice"); err != nil {
		t.Fatalf("Add a->c: %v", err)
	}

	links, _ := sdk.Links.List("a", "out")
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}

	labels := map[string]bool{}
	for _, l := range links {
		labels[l.Label] = true
	}
	if !labels["blocks"] || !labels["relates"] {
		t.Errorf("labels = %v, want blocks and relates", labels)
	}
}
