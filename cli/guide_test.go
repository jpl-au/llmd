package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/guide"
	"github.com/jpl-au/llmd/sdk"
)

// guidePageFor maps a command to its guide page when the names differ.
var guidePageFor = map[string]string{"hook": "hooks"}

// TestGuideShape is the guard for the guide pages' dense-first shape:
// a title, then one plain paragraph saying what the page covers, before
// any heading, table or code. A command's page also has a Usage section
// that shows the command, and names every flag the command's spec
// declares, so the page cannot drift from the code.
func TestGuideShape(t *testing.T) {
	names, err := guide.List()
	if err != nil {
		t.Fatal(err)
	}
	names = append(names, "guide")
	cmds := (&CLI{}).Commands()
	pages := make(map[string]bool, len(names))
	for _, name := range names {
		pages[name] = true
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			text, err := guide.Get(name)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(text, "\n")
			if !strings.HasPrefix(lines[0], "# ") {
				t.Errorf("first line = %q, want a # title", lines[0])
			}
			summary := ""
			for _, l := range lines[1:] {
				if strings.TrimSpace(l) == "" {
					if summary != "" {
						break
					}
					continue
				}
				summary += l
			}
			switch {
			case summary == "":
				t.Error("no summary paragraph after the title")
			case strings.HasPrefix(summary, "#"), strings.HasPrefix(summary, "|"), strings.HasPrefix(summary, "```"), strings.HasPrefix(summary, "- "):
				t.Errorf("summary = %q, want a plain sentence before any heading, table, code or list", summary)
			}
		})
	}

	for _, cmd := range cmds {
		page := cmd.Name
		if p, ok := guidePageFor[page]; ok {
			page = p
		}
		if !pages[page] {
			continue
		}
		t.Run("command/"+cmd.Name, func(t *testing.T) {
			text, err := guide.Get(page)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(text, "\n## Usage\n") {
				t.Error("no ## Usage section")
			}
			if !strings.Contains(text, "llmd "+cmd.Name) {
				t.Errorf("never shows `llmd %s`", cmd.Name)
			}
			for _, f := range cmd.Flags {
				if !mentionsFlag(text, f) {
					t.Errorf("flag --%s is in the spec but not on the page", f.Name)
				}
			}
		})
	}
}

// mentionsFlag reports whether a page names the flag in any of its
// spellings: --name, -s for a short form, or -x for a one-letter name.
func mentionsFlag(text string, f sdk.Flag) bool {
	if strings.Contains(text, "--"+f.Name) {
		return true
	}
	if f.Short != "" && strings.Contains(text, "-"+f.Short) {
		return true
	}
	return len(f.Name) == 1 && strings.Contains(text, "-"+f.Name)
}

// TestGuideFlagsExist is the other direction of TestGuideShape: a
// long flag a command page documents for its own command must exist
// in the spec, so a renamed or removed flag does not live on in the
// guide. Flags inside bash blocks are checked too, since those are
// the lines agents copy.
func TestGuideFlagsExist(t *testing.T) {
	flagRe := regexp.MustCompile(`--([a-z][a-z-]*)`)
	for _, cmd := range (&CLI{}).Commands() {
		page := cmd.Name
		if p, ok := guidePageFor[page]; ok {
			page = p
		}
		data, err := os.ReadFile(filepath.Join("..", "guide", page+".md"))
		if err != nil {
			continue // no page for this command
		}
		known := map[string]bool{"author": true, "json": true, "db": true, "help": true, "verbose": true, "global": true}
		for _, f := range cmd.Flags {
			known[f.Name] = true
		}
		for _, f := range subcommandFlags[cmd.Name] {
			known[f] = true
		}
		t.Run(cmd.Name, func(t *testing.T) {
			for line := range strings.SplitSeq(string(data), "\n") {
				// Only lines that invoke this very command are the agent's
				// copy source; other commands' flags may appear in prose.
				if !strings.Contains(line, "llmd "+cmd.Name+" ") && !strings.Contains(line, "llmd --author") {
					continue
				}
				if !strings.Contains(line, " "+cmd.Name+" ") {
					continue
				}
				for _, m := range flagRe.FindAllStringSubmatch(line, -1) {
					if !known[m[1]] {
						t.Errorf("line %q uses --%s, which %s does not have", line, m[1], cmd.Name)
					}
				}
			}
		})
	}
}

// subcommandFlags lists flags that belong to a subcommand rather than
// the command's top-level spec, so pages may use them in examples. Kept
// by hand from the per-subcommand flag lists in cli/<command>*.go.
var subcommandFlags = map[string][]string{
	"task":  {"column", "priority", "assign", "branch", "depends-on", "path", "file", "flag", "unflag", "position", "after", "base", "stat", "from", "to", "all", "title", "since", "budget", "name", "n"},
	"audit": {"status", "assign", "file", "version", "by-author", "pending", "since", "all"},
	"queue": {"assign", "limit", "since", "n", "all"},
	"rule":  {"agent", "role", "success", "failure", "resume"},
	"agent": {"command", "status", "task", "agent", "exit-code", "worktree"},
}
