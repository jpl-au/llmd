package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// TestTaskNext covers the claim through the CLI: the first ready task
// of up-next moves to in-progress assigned to the author and comes
// back in full, tasks flagged human are passed over, the columns and
// assignee can be chosen, an empty column is an error that names it,
// and a claim needs an author. Each case seeds Alpha and Beta in
// up-next (Alpha first), Gamma flagged human ahead of both, and Delta
// in review.
func TestTaskNext(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		drain       int // claims made before the one under test
		args        []string
		wantTitle   string
		wantStatus  string
		wantAssign  string
		wantText    []string
		wantErr     error
		wantErrText []string
	}{
		{
			name:       "claims the first ready task",
			args:       []string{"next"},
			wantTitle:  "Alpha",
			wantStatus: "in-progress",
			wantAssign: "worker",
			wantText:   []string{"Claimed", "from up-next to in-progress", "assigned to worker", "# Alpha", "| Status | in-progress |", "Alpha spec body."},
		},
		{
			name:       "the second claim takes the next task",
			drain:      1,
			args:       []string{"next"},
			wantTitle:  "Beta",
			wantStatus: "in-progress",
			wantAssign: "worker",
		},
		{
			name:        "an empty column is an error naming the column",
			drain:       2,
			args:        []string{"next"},
			wantErr:     sdk.ErrNoneReady,
			wantErrText: []string{"no task ready in up-next"},
		},
		{
			name:       "columns and assignee are flags",
			args:       []string{"next", "--from", "review", "--to", "approval", "--assign", "reviewer"},
			wantTitle:  "Delta",
			wantStatus: "approval",
			wantAssign: "reviewer",
			wantText:   []string{"from review to approval", "assigned to reviewer"},
		},
		{
			name:        "unknown column",
			args:        []string{"next", "--from", "nope"},
			wantErr:     sdk.ErrInvalidArg,
			wantErrText: []string{"nope"},
		},
		{name: "needs an author", noAuthor: true, args: []string{"next"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", args: []string{"next", "--bogus"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := host.TestSetup(t, host.TestIsolated)
			seed := h.Context(t.Context(), "alice")
			gamma := taskSeed(t, seed, "Gamma", taskBody("Gamma"), "--column", "up-next")
			run(t, seed, "task", nil, "set", gamma.Key, "--flag", "human")
			taskSeed(t, seed, "Alpha", taskBody("Alpha"), "--column", "up-next")
			taskSeed(t, seed, "Beta", taskBody("Beta"), "--column", "up-next")
			taskSeed(t, seed, "Delta", taskBody("Delta"), "--column", "review")

			author := "worker"
			if tt.noAuthor {
				author = ""
			}
			c := h.Context(t.Context(), author)
			for range tt.drain {
				run(t, c, "task", nil, "next")
			}

			if tt.wantErr != nil {
				err := runErr(t, c, "task", nil, tt.args...)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("task %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				for _, want := range tt.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("task %v error = %q, want to contain %q", tt.args, err, want)
					}
				}
				return
			}

			resp := run(t, c, "task", nil, tt.args...)
			got, ok := data(resp).(*sdk.Task)
			if !ok {
				t.Fatalf("task %v Data = %T, want *sdk.Task", tt.args, data(resp))
			}
			if got.Title != tt.wantTitle || got.Status != tt.wantStatus || got.AssignedTo != tt.wantAssign {
				t.Errorf("task %v = %q in %s assigned to %q, want %q in %s assigned to %q",
					tt.args, got.Title, got.Status, got.AssignedTo, tt.wantTitle, tt.wantStatus, tt.wantAssign)
			}
			stored := taskRead(t, c, got.Key)
			if stored.Status != tt.wantStatus || stored.AssignedTo != tt.wantAssign {
				t.Errorf("stored task = %s/%q, want %s/%q", stored.Status, stored.AssignedTo, tt.wantStatus, tt.wantAssign)
			}
			if human := taskRead(t, c, gamma.Key); human.Status != "up-next" {
				t.Errorf("human task moved to %s, want left in up-next", human.Status)
			}
			taskWantText(t, "task", tt.args, text(resp), tt.wantText, nil)
		})
	}
}

// TestTaskListDoneCap checks that the board view shows only the newest
// done tasks, says how many it left out, and shows them all with --all
// or when the done column is asked for directly.
func TestTaskListDoneCap(t *testing.T) {
	const extra = 3
	h := host.TestSetup(t, host.TestIsolated)
	seed := h.Context(t.Context(), "alice")
	var titles []string
	for i := range doneShown + extra {
		title := "Done " + string(rune('A'+i))
		titles = append(titles, title)
		taskSeed(t, seed, title, taskBody(title), "--column", "done")
	}
	taskSeed(t, seed, "Open", taskBody("Open"), "--column", "up-next")
	oldest, newest := titles[:extra], titles[extra:]

	tests := []struct {
		name       string
		args       []string
		wantText   []string
		wantAbsent []string
		wantCount  int
	}{
		{
			name:       "board view keeps the newest done tasks",
			args:       []string{"list"},
			wantText:   append([]string{"Open", "3 more done; use --all or --column done"}, newest...),
			wantAbsent: oldest,
			wantCount:  doneShown + 1,
		},
		{
			name:       "--all lists every done task",
			args:       []string{"list", "--all"},
			wantText:   append([]string{"Open"}, titles...),
			wantAbsent: []string{"more done"},
			wantCount:  doneShown + extra + 1,
		},
		{
			name:       "the done column itself is not capped",
			args:       []string{"list", "--column", "done"},
			wantText:   titles,
			wantAbsent: []string{"Open", "more done"},
			wantCount:  doneShown + extra,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := run(t, seed, "task", nil, tt.args...)
			taskWantText(t, "task", tt.args, text(resp), tt.wantText, tt.wantAbsent)
			tasks, ok := data(resp).([]*sdk.Task)
			if !ok {
				t.Fatalf("task %v Data = %T, want []*sdk.Task", tt.args, data(resp))
			}
			if len(tasks) != tt.wantCount {
				t.Errorf("task %v Data has %d tasks, want %d", tt.args, len(tasks), tt.wantCount)
			}
		})
	}
}
