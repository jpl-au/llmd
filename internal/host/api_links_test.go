package host

import (
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func TestLinksAdd(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a", []byte("x"), "alice", "")
	sdk.Documents.Write("b", []byte("x"), "alice", "")

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

	sdk.Documents.Write("x", []byte("x"), "alice", "")
	sdk.Documents.Write("y", []byte("y"), "alice", "")

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

	sdk.Documents.Write("a", []byte("x"), "alice", "")
	sdk.Documents.Write("b", []byte("x"), "alice", "")
	sdk.Links.Add("a", "b", "", "alice")

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

	sdk.Documents.Write("center", []byte("x"), "alice", "")
	sdk.Documents.Write("out1", []byte("x"), "alice", "")
	sdk.Documents.Write("out2", []byte("x"), "alice", "")

	sdk.Links.Add("center", "out1", "", "alice")
	sdk.Links.Add("center", "out2", "", "alice")

	links, _ := sdk.Links.List("center", "out")
	if len(links) != 2 {
		t.Errorf("outgoing: got %d, want 2", len(links))
	}
}

func TestLinksListIncoming(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("target", []byte("x"), "alice", "")
	sdk.Documents.Write("src1", []byte("x"), "alice", "")
	sdk.Documents.Write("src2", []byte("x"), "alice", "")

	sdk.Links.Add("src1", "target", "", "alice")
	sdk.Links.Add("src2", "target", "", "alice")

	links, _ := sdk.Links.List("target", "in")
	if len(links) != 2 {
		t.Errorf("incoming: got %d, want 2", len(links))
	}
}

func TestLinksListBoth(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a", []byte("x"), "alice", "")
	sdk.Documents.Write("b", []byte("x"), "alice", "")
	sdk.Documents.Write("c", []byte("x"), "alice", "")

	sdk.Links.Add("a", "b", "", "alice")
	sdk.Links.Add("c", "b", "", "alice")

	links, _ := sdk.Links.List("b", "both")
	if len(links) != 2 {
		t.Errorf("both directions: got %d, want 2", len(links))
	}
}

func TestLinksListEmpty(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("lonely", []byte("x"), "alice", "")

	links, err := sdk.Links.List("lonely", "out")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("got %d links, want 0", len(links))
	}
}
