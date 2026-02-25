// task_link.go handles linking tasks to documents.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskLink creates a link from a task's spec document to another
// document in the store. Requires two arguments: the task key and the
// target document path. The link is created from the task's spec path,
// not from the task key itself.
func taskLink(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task link: %w: id and path", sdk.ErrMissingArg)
	}

	// Look up the task to get its document path
	t, err := sdk.Tasks.Read(args[0])
	if err != nil {
		return nil, fmt.Errorf("task link: %w", err)
	}

	if err := sdk.Links.Add(t.Path, args[1], "", ctx.Author); err != nil {
		return nil, fmt.Errorf("task link: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Linked task %s to %s", args[0], args[1])), nil
}

// taskLinks lists outgoing links from a task's spec document. Shows
// each linked document path with its label (if any).
func taskLinks(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task links: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(args[0])
	if err != nil {
		return nil, fmt.Errorf("task links: %w", err)
	}

	links, err := sdk.Links.List(t.Path, "out")
	if err != nil {
		return nil, fmt.Errorf("task links: %w", err)
	}

	var lines []string
	for _, l := range links {
		if l.Label != "" {
			lines = append(lines, fmt.Sprintf("%s (%s)", l.To, l.Label))
		} else {
			lines = append(lines, l.To)
		}
	}
	return sdk.Result{Text: strings.Join(lines, "\n"), Data: links}, nil
}
