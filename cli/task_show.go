// task_show.go displays a single task's detail view.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskShow displays a single task's metadata and spec body. Renders a
// markdown document with a metadata table (ID, status, priority,
// assignee, branch, flags, spec path) followed by the spec document
// content if it exists.
func taskShow(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task show: %w: id", sdk.ErrMissingArg)
	}

	t, err := ctx.Tasks.Read(args[0])
	if err != nil {
		return nil, fmt.Errorf("task show: %w", err)
	}

	// Read the document body
	body, err := ctx.Documents.Read(t.Path, 0)
	if err != nil {
		body = nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	fmt.Fprintf(&b, "| Field | Value |\n")
	fmt.Fprintf(&b, "|-------|-------|\n")
	fmt.Fprintf(&b, "| ID | %s |\n", t.Key)
	fmt.Fprintf(&b, "| Status | %s |\n", t.Status)
	fmt.Fprintf(&b, "| Priority | %d |\n", t.Priority)
	if t.AssignedTo != "" {
		fmt.Fprintf(&b, "| Assigned To | %s |\n", t.AssignedTo)
	}
	if t.Branch != "" {
		branchVal := t.Branch
		if ctx.Git.Available() == nil {
			if base, err := ctx.Git.DefaultBranch(); err == nil {
				if ahead, behind, err := ctx.Git.RevCount(base, t.Branch); err == nil {
					branchVal = fmt.Sprintf("%s (+%d/-%d)", t.Branch, ahead, behind)
				}
			}
		}
		fmt.Fprintf(&b, "| Branch | %s |\n", branchVal)
	}
	if t.DependsOn != "" {
		fmt.Fprintf(&b, "| Depends On | %s |\n", t.DependsOn)
	}
	if t.Flags != "" {
		fmt.Fprintf(&b, "| Flags | %s |\n", t.Flags)
	}
	fmt.Fprintf(&b, "| Spec | %s |\n", t.Path)
	fmt.Fprintf(&b, "\n---\n\n")

	if body != nil {
		b.Write(body)
	}

	return sdk.Result{Text: b.String(), Data: t}, nil
}
