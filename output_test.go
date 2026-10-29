package main

import "testing"

// TestHighlightCommands checks which words of an error message are
// coloured as the command to run.
func TestHighlightCommands(t *testing.T) {
	c := func(s string) string { return commandStyle.Render(s) }
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{"ends the sentence", "task start: unable to spawn agents. Please run llmd serve.",
			"task start: unable to spawn agents. Please run " + c("llmd serve") + "."},
		{"followed by to", "there is no store here. Run llmd init to create one.",
			"there is no store here. Run " + c("llmd init") + " to create one."},
		{"with an argument", "cat: p is deleted. Run llmd restore p to bring it back.",
			"cat: p is deleted. Run " + c("llmd restore p") + " to bring it back."},
		{"followed by first", "mirror push: no mirror. Run llmd mirror pull first.",
			"mirror push: no mirror. Run " + c("llmd mirror pull") + " first."},
		{"two commands", "Write one with llmd write x, or link one with llmd task link k <path>.",
			"Write one with " + c("llmd write x") + ", or link one with " + c("llmd task link k <path>") + "."},
		{"prefix is not a command", "llmd: interrupted.", "llmd: interrupted."},
		{"store path is not a command", "there is no store at .llmd/llmd.db.", "there is no store at .llmd/llmd.db."},
		{"no command", "cat: p does not exist.", "cat: p does not exist."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := highlightCommands(tt.msg); got != tt.want {
				t.Errorf("highlightCommands(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}
