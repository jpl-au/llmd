package cli

// diff compares two document versions.
//
// With two paths: compares them directly ("diff notes/a notes/b").
// With one path: compares the current version to its immediate predecessor,
// which is the common "what changed last?" use case. It fetches the two
// most recent versions from history to construct the comparison.
//
// Paths can include version suffixes ("notes/a:3") to compare specific
// versions. See sdk.DocumentStore.Diff for the path:version syntax.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// defaultDiffMaxLines caps diff output for the AI-first common case.
// A huge rewrite diff can easily produce thousands of lines, which
// blows an agent's context window for little marginal value - the
// first few hundred lines plus the stat line are enough to understand
// what changed. --all overrides.
const defaultDiffMaxLines = 500

var diffSpec = sdk.Command{
	Name: "diff", Desc: `Compare document versions or two documents

With one path, diffs the latest version against the previous. With
two paths (optionally using :version suffix), compares them directly.
Paths and document keys are interchangeable. Output is coloured in
a terminal.

Diffs over 500 lines are truncated with a summary footer so agents
do not burn context on large rewrites. Pass --all to see the whole
diff, or --stat to see just the +/- counts.

Examples:
  llmd diff notes/meeting              Latest vs previous version
  llmd diff notes/meeting:2 notes/meeting:5   Version 2 vs 5
  llmd diff abc123def:2 abc123def:5    Same, using the document key
  llmd diff notes/a notes/b            Two different documents`, Usage: "diff [flags] <source> [target]", MCP: true, Flags: []sdk.Flag{
		{Name: "C", Type: "int", Desc: "Lines of context"},
		{Name: "stat", Type: "bool", Desc: "Show stats only"},
		{Name: "all", Type: "bool", Desc: "Show full diff, no line cap"},
	},
}

// diffCmd compares document versions and displays a unified diff.
func diffCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, paths, err := sdk.ParseArgs(diffSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}
	contextLines := flags.Int("C")
	statOnly := flags.Bool("stat")
	showAll := flags.Bool("all")

	if len(paths) == 0 {
		return nil, fmt.Errorf("diff: %w", sdk.ErrMissingArg)
	}

	var source, target string
	switch len(paths) {
	case 1:
		// Single path: compare current version to its predecessor.
		versions, err := ctx.Documents.History(paths[0], 2)
		if err != nil || len(versions) < 2 {
			return nil, fmt.Errorf("diff: no previous version for %s", paths[0])
		}
		source = paths[0] + ":" + strconv.Itoa(versions[1].Number)
		target = paths[0]
	case 2:
		source, target = paths[0], paths[1]
	default:
		return nil, fmt.Errorf("diff: %w: expected 1 or 2 paths, got %d", sdk.ErrInvalidArg, len(paths))
	}

	diffText, added, removed, err := ctx.Documents.Diff(source, target, contextLines)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if statOnly {
		return sdk.Text(fmt.Sprintf("+%d -%d", added, removed)), nil
	}

	if diffText == "" {
		return sdk.Text("No differences"), nil
	}

	// Cap by default so huge diffs don't blow agent context windows.
	// The footer tells the reader exactly how much was dropped and
	// how to see the rest.
	if !showAll {
		if capped, total := truncateDiff(diffText, defaultDiffMaxLines); capped != diffText {
			diffText = fmt.Sprintf("%s\n... %d more lines truncated (use --all to show full diff, +%d -%d total)",
				capped, total-defaultDiffMaxLines, added, removed)
		}
	}

	if isTTY() {
		diffText = colourDiff(diffText)
	}

	return sdk.Text(diffText), nil
}

// truncateDiff returns the first max lines of a diff along with the
// total line count. If the diff is at or under the cap, the original
// text is returned unchanged and the caller can compare identity to
// detect the no-op case.
func truncateDiff(text string, max int) (string, int) {
	lines := strings.Split(text, "\n")
	if len(lines) <= max {
		return text, len(lines)
	}
	return strings.Join(lines[:max], "\n"), len(lines)
}

// colourDiff applies terminal colours to unified diff output.
func colourDiff(text string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			b.WriteString(diffHeader.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(diffHunk.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(diffAdded.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(diffRemoved.Render(line))
		default:
			b.WriteString(line)
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
