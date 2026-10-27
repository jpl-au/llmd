package tasks

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/pkg/model/core"
)

// seedOrdered creates, in up-next and in this creation order, a task
// with no priority, one at priority 3, one at priority 1 and a second
// at priority 3, so creation order, priority order and position order
// all differ.
func seedOrdered(t *testing.T, ts *Tasks) {
	t.Helper()
	origin := core.Origin{Author: "alice", Source: "test"}
	spec := []byte("# Spec\n\nbody")
	for _, tk := range []struct {
		title string
		pri   int
	}{{"None", 0}, {"Low", 3}, {"High", 1}, {"Low too", 3}} {
		if _, err := ts.Add(t.Context(), tk.title, spec, AddOptions{Origin: origin, Status: "up-next", Priority: tk.pri}); err != nil {
			t.Fatalf("Add(%s): %v", tk.title, err)
		}
	}
}

// TestOrder covers the two orders within a column, as List shows them
// and as Claim takes them: by default priority 1 comes first, equal
// priorities keep board position, and no priority comes last; with
// OrderPosition the board position alone decides.
func TestOrder(t *testing.T) {
	tests := []struct {
		name  string
		order Order
		want  []string
	}{
		{"priority is the default", "", []string{"High", "Low", "Low too", "None"}},
		{"priority", OrderPriority, []string{"High", "Low", "Low too", "None"}},
		{"position", OrderPosition, []string{"None", "Low", "High", "Low too"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			if err := ts.SetOrder(tt.order); err != nil {
				t.Fatal(err)
			}
			seedOrdered(t, ts)

			listed, err := ts.List(t.Context(), ListOptions{Status: "up-next"})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, tk := range listed {
				got = append(got, tk.Title)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("List() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("List()[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}

			// Claim walks the same order, one task per call.
			for _, want := range tt.want {
				tk, err := ts.Claim(t.Context(), "worker", ClaimOptions{})
				if err != nil {
					t.Fatalf("Claim() = %v, want %q", err, want)
				}
				if tk.Title != want {
					t.Errorf("Claim() = %q, want %q", tk.Title, want)
				}
			}
		})
	}
}

// TestSetOrder rejects an unknown order and accepts the zero value.
func TestSetOrder(t *testing.T) {
	ts := setup(t)
	for _, ok := range []Order{"", OrderPriority, OrderPosition} {
		if err := ts.SetOrder(ok); err != nil {
			t.Errorf("SetOrder(%q) = %v, want nil", ok, err)
		}
	}
	if err := ts.SetOrder("random"); !errors.Is(err, ErrInvalidOrder) {
		t.Errorf("SetOrder(random) = %v, want ErrInvalidOrder", err)
	}
}

// TestPriorityValidation refuses a negative priority on Add and on
// Set, and accepts zero and positive values.
func TestPriorityValidation(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}
	ts := setup(t)
	if _, err := ts.Add(t.Context(), "Negative", nil, AddOptions{Origin: origin, Priority: -1}); !errors.Is(err, ErrInvalidPriority) {
		t.Errorf("Add(priority -1) error = %v, want ErrInvalidPriority", err)
	}
	tk, err := ts.Add(t.Context(), "Fine", nil, AddOptions{Origin: origin, Priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.Set(t.Context(), tk.Key, "alice", SetOptions{Priority: new(-5)}); !errors.Is(err, ErrInvalidPriority) {
		t.Errorf("Set(priority -5) error = %v, want ErrInvalidPriority", err)
	}
	if err := ts.Set(t.Context(), tk.Key, "alice", SetOptions{Priority: new(0)}); err != nil {
		t.Errorf("Set(priority 0) error = %v, want nil (clears the priority)", err)
	}
	got, err := ts.Read(t.Context(), tk.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != 0 {
		t.Errorf("Priority after Set(0) = %d, want 0", got.Priority)
	}
}
