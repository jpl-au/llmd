package cli

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestErrorStyle holds error messages to one style, whichever command
// produces them: "<command>: <what is wrong>." and, where there is
// one, a second sentence with the next step. The message names the
// thing once and says nothing about the code that found the problem:
// no chains of "x: y: z", no " - " asides, no sentinel text such as
// "not found: document not found".
//
// Each case is a command that must fail. A store is set up with a live
// document p (two versions), a live document o, a deleted document gone
// and a task with no spec; {task} in an argument is that task's key.
func TestErrorStyle(t *testing.T) {
	tests := []struct {
		cmd    string
		args   []string
		stdin  string
		author string // "-" runs with no author
	}{
		{cmd: "cat", args: []string{"nothere"}},
		{cmd: "cat", args: []string{"gone"}},
		{cmd: "cat", args: []string{"p:9"}},
		{cmd: "cat", args: []string{"--bogus", "p"}},
		{cmd: "write", args: []string{"p"}},
		{cmd: "write", args: []string{"p"}, stdin: "x", author: "-"},
		{cmd: "write", args: []string{"../escape"}, stdin: "x"},
		{cmd: "edit", args: []string{"nothere", "a", "b"}},
		{cmd: "edit", args: []string{"gone", "a", "b"}},
		{cmd: "edit", args: []string{"p", "absent text", "b"}},
		{cmd: "edit", args: []string{"p", "two", "b"}},
		{cmd: "edit", args: []string{"p", "and", "and"}},
		{cmd: "sed", args: []string{"-i", "s/a/b/", "nothere"}},
		{cmd: "sed", args: []string{"-i", "nonsense", "p"}},
		{cmd: "rm", args: []string{"nothere"}},
		{cmd: "rm", args: []string{"gone"}},
		{cmd: "mv", args: []string{"nothere", "x"}},
		{cmd: "mv", args: []string{"gone", "x"}},
		{cmd: "mv", args: []string{"p", "o"}},
		{cmd: "mv", args: []string{"o", "gone"}},
		{cmd: "restore", args: []string{"p"}},
		{cmd: "restore", args: []string{"nothere"}},
		{cmd: "revert", args: []string{"p", "9"}},
		{cmd: "revert", args: []string{"gone", "1"}},
		{cmd: "revert", args: []string{"nothere", "1"}},
		{cmd: "history", args: []string{"nothere"}},
		{cmd: "history", args: []string{"gone"}},
		{cmd: "diff", args: []string{"nothere:1", "p:1"}},
		{cmd: "diff", args: []string{"p:1", "p:9"}},
		{cmd: "tag", args: []string{"gone", "red"}},
		{cmd: "tag", args: []string{"nothere", "red"}},
		{cmd: "tag", args: []string{"p", "Bad Name!"}},
		{cmd: "link", args: []string{"p", "p"}},
		{cmd: "link", args: []string{"p", "gone"}},
		{cmd: "link", args: []string{"nothere", "p"}},
		{cmd: "unlink", args: []string{"p", "o"}},
		{cmd: "ls", args: []string{"--bogus"}},
		{cmd: "find", args: []string{`"`}},
		{cmd: "glob", args: []string{"["}},
		{cmd: "export", args: []string{"nothere", "{dir}"}},
		{cmd: "export", args: []string{"gone", "{dir}"}},
		{cmd: "import", args: []string{"{dir}/missing"}},
		{cmd: "task", args: []string{"bogus"}},
		{cmd: "task", args: []string{"show", "zzzzzzzzzz"}},
		{cmd: "task", args: []string{"move", "zzzzzzzzzz", "done"}},
		{cmd: "task", args: []string{"move", "{task}", "nowhere"}},
		{cmd: "task", args: []string{"move", "{task}", "done"}},
		{cmd: "task", args: []string{"start", "zzzzzzzzzz"}},
		{cmd: "task", args: []string{"add", "Spec", "--path", "gone"}},
		{cmd: "task", args: []string{"add", "Spec", "--path", "nothere"}},
		{cmd: "task", args: []string{"add", "Bad", "--priority", "-1"}},
		{cmd: "task", args: []string{"column", "add", "backlog"}},
		{cmd: "task", args: []string{"add", "x"}, author: "-"},
		{cmd: "queue", args: []string{"send"}},
		{cmd: "queue", args: []string{"ack", "zzzzzzzzzz"}},
		{cmd: "audit", args: []string{"show", "zzzzzzzzzz"}},
		{cmd: "audit", args: []string{"reply", "zzzzzzzzzz", "text"}},
		{cmd: "audit", args: []string{"resolve", "zzzzzzzzzz"}},
		{cmd: "agent", args: []string{"stop", "zzzzzzzzzz"}},
		{cmd: "vacuum", args: []string{"--older-than", "soon"}},
		{cmd: "hook", args: []string{}},
		{cmd: "hook", args: []string{}, stdin: "not json"},
		{cmd: "guide", args: []string{"nothing-here"}},
		{cmd: "cat"},
		{cmd: "mv", args: []string{"p"}},
		{cmd: "revert", args: []string{"p", "two"}},
		{cmd: "diff", args: []string{"p", "o", "gone"}},
		{cmd: "tag", args: []string{"p", "red"}, author: "-"},
		{cmd: "tag", args: []string{"-d", "p", "blue"}},
		{cmd: "link", args: []string{"o", "p"}},
		{cmd: "task", args: []string{}},
		{cmd: "task", args: []string{"set", "{task}", "--priority", "-2"}},
		{cmd: "task", args: []string{"start", "{task}", "--agent", "x", "--assign", "y"}},
		{cmd: "task", args: []string{"column", "rm", "backlog"}},
		{cmd: "task", args: []string{"column", "rm", "nowhere"}},
		{cmd: "task", args: []string{"column", "bogus"}},
		{cmd: "audit", args: []string{"bogus"}},
		{cmd: "queue", args: []string{"bogus"}},
		{cmd: "rule", args: []string{"bogus"}},
		{cmd: "agent", args: []string{"bogus"}},
		{cmd: "agent", args: []string{"add", "nosuchagent"}},
		{cmd: "mirror", args: []string{"bogus"}},
		{cmd: "hook", args: []string{"init"}},
		{cmd: "config", args: []string{"a", "b", "c"}},
		{cmd: "config", args: []string{"limits.path_length", "0"}},
		{cmd: "config", args: []string{"server.read_timeout", "soon"}},
		{cmd: "config", args: []string{"no.such.key", "x"}},
		{cmd: "nosuchcommand", args: []string{}},
	}

	c := newCLI(t, "alice")
	writeDoc(t, c, "p", "one")
	writeDoc(t, c, "p", "two and two")
	writeDoc(t, c, "o", "other")
	writeDoc(t, c, "gone", "gone")
	run(t, c, "rm", nil, "gone")
	run(t, c, "link", nil, "o", "p")
	bare, err := c.Tasks.Add("No spec", nil, sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Tasks.Add() error = %v", err)
	}
	taskKey := bare.Key
	dir := t.TempDir()

	for _, tt := range tests {
		args := make([]string, len(tt.args))
		for i, a := range tt.args {
			a = strings.ReplaceAll(a, "{task}", taskKey)
			args[i] = strings.ReplaceAll(a, "{dir}", dir)
		}
		name := strings.TrimSpace(tt.cmd + " " + strings.Join(tt.args, " "))
		t.Run(name, func(t *testing.T) {
			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			var stdin []byte
			if tt.stdin != "" {
				stdin = []byte(tt.stdin)
			}
			_, err := c.Runtime.Dispatch(t.Context(), tt.cmd, args, author, stdin, "")
			if err == nil {
				t.Fatalf("llmd %s succeeded, want an error", name)
			}
			msg := strings.ReplaceAll(err.Error(), dir, "{dir}")
			if os.Getenv("LLMD_ERRSTYLE_LOG") != "" {
				t.Logf("llmd %-36s => %s", name, msg)
			}
			for _, p := range styleProblems(msg) {
				t.Errorf("llmd %s => %q: %s", name, msg, p)
			}
		})
	}
}

var (
	// errPrefix is the command (and subcommand) that leads a message.
	errPrefix = regexp.MustCompile(`^[a-z]+( [a-z]+){0,2}: `)
	// errDeep is a second "x: " after the command: a stacked phrase.
	errDeep = regexp.MustCompile(`[A-Za-z0-9)"']: `)
)

// styleProblems lists how msg departs from the error style.
func styleProblems(msg string) []string {
	var problems []string
	prefix := errPrefix.FindString(msg)
	if prefix == "" {
		problems = append(problems, `does not start with "<command>: "`)
	}
	body := strings.TrimPrefix(msg, prefix)
	if errDeep.MatchString(body) {
		problems = append(problems, `stacks phrases with ": " after the command`)
	}
	if strings.Contains(body, " - ") {
		problems = append(problems, `uses " - " for an aside; write a second sentence`)
	}
	if !strings.HasSuffix(body, ".") {
		problems = append(problems, "does not end with a full stop")
	}
	if strings.Count(body, ". ") > 1 {
		problems = append(problems, "has more than two sentences")
	}
	return problems
}
