package sdk

import (
	"strings"
	"testing"
)

// TestCommandSummary checks that Summary is the first paragraph of
// Desc, trimmed, and the whole of a Desc with no blank line.
func TestCommandSummary(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want string
	}{
		{"one line", "Read a document", "Read a document"},
		{"two paragraphs", "Read a document\n\nMore about it.", "Read a document"},
		{"two-line first paragraph", "Read a document\nfrom the store\n\nMore.", "Read a document\nfrom the store"},
		{"surrounding space", "  \nRead a document\n\n", "Read a document"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Command{Desc: tt.desc}
			if got := c.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCommandAgentHelp covers the agent rendering: Summary plus the
// flags for an ordinary command, the Brief alone for a command that
// wrote one, the usage line when present, flag types and short forms,
// and the guide pointer on every command.
func TestCommandAgentHelp(t *testing.T) {
	tests := []struct {
		name       string
		cmd        Command
		want       []string
		wantAbsent []string
	}{
		{
			name: "summary, usage, flags and pointer",
			cmd: Command{Name: "cat", Desc: "Read a document\n\nLong prose.", Usage: "cat [options] <path>", Flags: []Flag{
				{Name: "version", Type: "int", Desc: "Read a version"},
				{Name: "n", Type: "bool", Desc: "Number lines"},
				{Name: "status", Short: "s", Type: "string", Desc: "Filter"},
			}},
			want: []string{
				"Read a document\nUsage: llmd cat [options] <path>\nFlags:",
				"\n  --version <int>  Read a version",
				"\n  --n  Number lines",
				"\n  -s, --status <string>  Filter",
				"\nDetails: guide cat",
			},
			wantAbsent: []string{"Long prose"},
		},
		{
			name: "a brief owns the text",
			cmd: Command{Name: "task", Desc: "Manage tasks\n\nProse.", Usage: "task <sub>", Brief: "Task board. Subcommands: list, next.", Flags: []Flag{
				{Name: "column", Type: "string", Desc: "Filter"},
			}},
			want:       []string{"Task board. Subcommands: list, next.\nUsage: llmd task <sub>\nDetails: guide task"},
			wantAbsent: []string{"Flags:", "--column", "Manage tasks", "Prose"},
		},
		{
			name:       "no usage and no flags",
			cmd:        Command{Name: "version", Desc: "Show the version"},
			want:       []string{"Show the version\nDetails: guide version"},
			wantAbsent: []string{"Usage:", "Flags:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cmd.AgentHelp()
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("AgentHelp() = %q, want to contain %q", got, w)
				}
			}
			for _, a := range tt.wantAbsent {
				if strings.Contains(got, a) {
					t.Errorf("AgentHelp() = %q, must not contain %q", got, a)
				}
			}
		})
	}
}
