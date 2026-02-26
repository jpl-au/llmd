package host

import (
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func TestActivitiesRecentEmpty(t *testing.T) {
	testHost(t)

	events, err := sdk.Activities.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events on empty store, want 0", len(events))
	}
}

func TestActivitiesRecentDocuments(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("hello"), "alice", "first")

	events, err := sdk.Activities.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Recent returned no events after document write")
	}

	found := false
	for _, e := range events {
		if e.Subject == "doc" && e.Author == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected activity for doc by alice")
	}
}

func TestActivitiesRecentTasks(t *testing.T) {
	testHost(t)

	_, err := sdk.Tasks.Add("test task", nil, sdk.TaskAddOpts{Author: "bob"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	events, err := sdk.Activities.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}

	found := false
	for _, e := range events {
		if e.Type == "task" && e.Author == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task activity by bob")
	}
}

func TestActivitiesRecentLimit(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a", []byte("x"), "alice", "")
	sdk.Documents.Write("b", []byte("x"), "alice", "")
	sdk.Documents.Write("c", []byte("x"), "alice", "")

	events, err := sdk.Activities.Recent(2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(events) > 2 {
		t.Errorf("got %d events with limit 2", len(events))
	}
}
