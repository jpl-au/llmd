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

// diffCmd compares document versions and displays a unified diff.
func diffCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var paths []string
	var contextLines int
	var statOnly bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case arg == "--stat":
			statOnly = true
		case len(arg) > 0 && arg[0] != '-':
			paths = append(paths, arg)
		}
	}

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

	if isTTY() {
		diffText = colourDiff(diffText)
	}

	return sdk.Text(diffText), nil
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
