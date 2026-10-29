package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// recorder collects the events a Claim emits.
type recorder struct{ events []events.Event }

func (r *recorder) HandleEvent(_ context.Context, e events.Event) error {
	r.events = append(r.events, e)
	return nil
}

// TestClaim covers the pick: board order within the source column,
// the flags and conditions that pass a task over, the defaults and
// overrides for columns and assignee, the exclusive hand-off between
// two claimers, and the errors. seed creates the board and returns the
// titles expected from successive claims, in order; an empty list
// means the first claim finds nothing.
func TestClaim(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}
	spec := []byte("# Spec\n\nDo the thing.")
	add := func(t *testing.T, ts *Tasks, title, status string, body []byte, flags ...string) string {
		t.Helper()
		tk, err := ts.Add(t.Context(), title, body, AddOptions{Origin: origin, Status: status})
		if err != nil {
			t.Fatalf("Add(%s): %v", title, err)
		}
		for _, f := range flags {
			if err := ts.Set(t.Context(), tk.Key, "alice", SetOptions{Flag: f}); err != nil {
				t.Fatalf("Set(%s, --flag %s): %v", title, f, err)
			}
		}
		return tk.Key
	}

	tests := []struct {
		name       string
		seed       func(t *testing.T, ts *Tasks)
		opts       ClaimOptions
		author     string
		want       []string // titles claimed by successive calls
		wantStatus string   // column of a claimed task; default in-progress
		wantAssign string   // assignee of a claimed task; default the author
		wantErr    error    // error of the first claim
	}{
		{
			name: "board order within the column",
			seed: func(t *testing.T, ts *Tasks) {
				add(t, ts, "First", "up-next", spec)
				add(t, ts, "Second", "up-next", spec)
				add(t, ts, "Elsewhere", "backlog", spec)
			},
			want: []string{"First", "Second"},
		},
		{
			name: "human, hold and blocked tasks are passed over",
			seed: func(t *testing.T, ts *Tasks) {
				add(t, ts, "Theirs", "up-next", spec, "human")
				add(t, ts, "Paused", "up-next", spec, "hold")
				add(t, ts, "Stuck", "up-next", spec, "blocked")
				add(t, ts, "Mine", "up-next", spec)
			},
			want: []string{"Mine"},
		},
		{
			name: "an unfinished dependency holds a task back",
			seed: func(t *testing.T, ts *Tasks) {
				dep := add(t, ts, "Dep", "in-progress", spec)
				if _, err := ts.Add(t.Context(), "Waiting", spec, AddOptions{Origin: origin, Status: "up-next", DependsOn: dep}); err != nil {
					t.Fatal(err)
				}
				add(t, ts, "Free", "up-next", spec)
			},
			want: []string{"Free"},
		},
		{
			name: "a done dependency releases a task",
			seed: func(t *testing.T, ts *Tasks) {
				dep := add(t, ts, "Dep", "done", spec)
				if _, err := ts.Add(t.Context(), "Released", spec, AddOptions{Origin: origin, Status: "up-next", DependsOn: dep}); err != nil {
					t.Fatal(err)
				}
			},
			want: []string{"Released"},
		},
		{
			name: "the backlog needs a spec",
			seed: func(t *testing.T, ts *Tasks) {
				add(t, ts, "Vague", "backlog", nil)
				add(t, ts, "Titled", "backlog", []byte("# Titled"))
				add(t, ts, "Specced", "backlog", spec)
			},
			opts: ClaimOptions{From: "backlog"},
			want: []string{"Specced"},
		},
		{
			name: "columns and assignee can be chosen",
			seed: func(t *testing.T, ts *Tasks) {
				add(t, ts, "Reviewed", "review", spec)
			},
			opts:       ClaimOptions{From: "review", To: "approval", AssignedTo: "bot"},
			want:       []string{"Reviewed"},
			wantStatus: "approval",
			wantAssign: "bot",
		},
		{
			name:    "nothing to take",
			seed:    func(t *testing.T, ts *Tasks) { add(t, ts, "Later", "backlog", spec) },
			wantErr: ErrNoneReady,
		},
		{
			name:    "unknown source column",
			seed:    func(t *testing.T, ts *Tasks) { add(t, ts, "Any", "up-next", spec) },
			opts:    ClaimOptions{From: "nope"},
			wantErr: ErrInvalidCol,
		},
		{
			name:    "unknown target column",
			seed:    func(t *testing.T, ts *Tasks) { add(t, ts, "Any", "up-next", spec) },
			opts:    ClaimOptions{To: "nope"},
			wantErr: ErrInvalidCol,
		},
		{
			name:    "source and target must differ",
			seed:    func(t *testing.T, ts *Tasks) { add(t, ts, "Any", "up-next", spec) },
			opts:    ClaimOptions{From: "review", To: "review"},
			wantErr: ErrInvalidCol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			tt.seed(t, ts)
			author := tt.author
			if author == "" {
				author = "worker"
			}

			if tt.wantErr != nil {
				_, err := ts.Claim(t.Context(), author, tt.opts)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Claim() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			wantStatus, wantAssign := tt.wantStatus, tt.wantAssign
			if wantStatus == "" {
				wantStatus = "in-progress"
			}
			if wantAssign == "" {
				wantAssign = author
			}
			for _, title := range tt.want {
				got, err := ts.Claim(t.Context(), author, tt.opts)
				if err != nil {
					t.Fatalf("Claim() = %v, want %q", err, title)
				}
				if got.Title != title || got.Status != wantStatus || got.AssignedTo != wantAssign {
					t.Errorf("Claim() = %q in %s assigned to %q, want %q in %s assigned to %q",
						got.Title, got.Status, got.AssignedTo, title, wantStatus, wantAssign)
				}
				stored, err := ts.Read(t.Context(), got.Key)
				if err != nil {
					t.Fatal(err)
				}
				if stored.Status != wantStatus || stored.AssignedTo != wantAssign {
					t.Errorf("stored task = %s/%q, want %s/%q", stored.Status, stored.AssignedTo, wantStatus, wantAssign)
				}
			}
			// The column is drained: the next claim finds nothing.
			if _, err := ts.Claim(t.Context(), author, tt.opts); !errors.Is(err, ErrNoneReady) {
				t.Errorf("Claim() after %d claims error = %v, want ErrNoneReady", len(tt.want), err)
			}
		})
	}
}

// TestClaimRecords checks what a claim leaves behind: a moved entry
// and an assignment entry in the task log, the task at the end of the
// target column, and one task.moved event marked as a claim.
func TestClaimRecords(t *testing.T) {
	ts := setup(t)
	rec := &recorder{}
	ts.bus.Subscribe(rec)
	origin := core.Origin{Author: "alice", Source: "test"}
	spec := []byte("# Spec\n\nbody")

	if _, err := ts.Add(t.Context(), "Busy", spec, AddOptions{Origin: origin, Status: "in-progress"}); err != nil {
		t.Fatal(err)
	}
	seeded, err := ts.Add(t.Context(), "Next", spec, AddOptions{Origin: origin, Status: "up-next", AssignedTo: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	rec.events = nil

	got, err := ts.Claim(t.Context(), "worker", ClaimOptions{})
	if err != nil {
		t.Fatalf("Claim(): %v", err)
	}
	if got.Key != seeded.Key {
		t.Fatalf("Claim() = %s, want %s", got.Key, seeded.Key)
	}
	if got.Position != 1 {
		t.Errorf("Position = %d, want 1 (after the task already in progress)", got.Position)
	}

	log, err := ts.Log(t.Context(), got.Key, 0)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, e := range log {
		actions = append(actions, e.Actor+":"+e.Action+":"+e.OldValue+">"+e.NewValue)
	}
	for _, want := range []string{"worker:moved:up-next>in-progress", "worker:edited:assigned_to:alice>worker"} {
		found := false
		for _, a := range actions {
			if a == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Log() = %v, want to contain %q", actions, want)
		}
	}

	if len(rec.events) != 1 {
		t.Fatalf("emitted %d events, want 1: %+v", len(rec.events), rec.events)
	}
	e := rec.events[0]
	if e.Type != events.TaskMoved || e.Key != got.Key || e.Author != "worker" {
		t.Errorf("event = %+v, want task.moved for %s by worker", e, got.Key)
	}
	if e.Metadata["from"] != "up-next" || e.Metadata["to"] != "in-progress" || e.Metadata["claimed"] != true || e.Metadata["assigned_to"] != "worker" {
		t.Errorf("event metadata = %v", e.Metadata)
	}
}
