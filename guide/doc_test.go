package guide

import (
	"slices"
	"testing"
	"testing/fstest"
)

// TestAdd checks that a page an extension adds is read and listed
// like llmd's own, and that a name already taken is refused.
func TestAdd(t *testing.T) {
	t.Cleanup(func() {
		mu.Lock()
		delete(added, "board-game")
		mu.Unlock()
	})
	Add(fstest.MapFS{
		"board-game.md": {Data: []byte("# Board game\n")},
		"notes.txt":     {Data: []byte("not a page")},
	})

	got, err := Get("board-game")
	if err != nil || got != "# Board game\n" {
		t.Errorf("Get(board-game) = %q, %v, want the added page", got, err)
	}
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, "board-game") || slices.Contains(names, "notes") {
		t.Errorf("List() = %v, want board-game and no notes", names)
	}
	if !slices.IsSorted(names) {
		t.Errorf("List() = %v, want it sorted", names)
	}

	for _, name := range []string{"cat", "board-game"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Add(%s.md) did not panic, want the taken name refused", name)
				}
			}()
			Add(fstest.MapFS{name + ".md": {Data: []byte("# Again\n")}})
		}()
	}
}
