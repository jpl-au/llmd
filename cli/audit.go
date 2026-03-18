// audit.go dispatches audit subcommands.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

var auditSpec = sdk.Command{
	Name: "audit", Desc: `Agent-to-agent and human-to-agent review threads.

Subcommands (passed as first arg):
  add <target> [content]     create top-level audit
  reply <id> [content]       reply to an existing thread
  list [target] [--since 5m] list audits (optionally filtered)
  show <id>                  full audit with thread
  resolve <id>               mark as approved (no body needed)
  rm <id>                    soft-delete
  restore <id>               recover a deleted audit
  status [--since 5m]        inbox: what needs my response`, Usage: "audit <subcommand> [options]", MCP: true, MCPName: "audit", Flags: []sdk.Flag{
		{Name: "status", Short: "s", Type: "string", Desc: "Set or filter by status"},
		{Name: "assign", Type: "string", Desc: "Assign to or filter by assigned person"},
		{Name: "file", Type: "string", Desc: "Read content from filesystem path"},
		{Name: "version", Type: "int", Desc: "Pin to specific document version"},
		{Name: "pending", Type: "bool", Desc: "Filter to pending/needs-work"},
		{Name: "by-author", Type: "string", Desc: "Filter by who created the audit"},
	},
}

// auditCmd dispatches to audit subcommands.
func auditCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audit: %w", sdk.ErrMissingArg)
	}

	sub := args[0]
	args = args[1:]

	// Author required for all mutations.
	switch sub {
	case "add", "reply", "resolve", "rm", "restore":
		if ctx.Author == "" {
			return nil, fmt.Errorf("audit %s: author required for mutations", sub)
		}
	case "status":
		if ctx.Author == "" {
			return nil, fmt.Errorf("audit status: author required")
		}
	}

	switch sub {
	case "add":
		return auditAdd(ctx, args)
	case "reply":
		return auditReply(ctx, args)
	case "list", "ls":
		return auditList(ctx, args)
	case "show":
		return auditShow(ctx, args)
	case "resolve":
		return auditResolve(ctx, args)
	case "rm":
		return auditRm(ctx, args)
	case "restore":
		return auditRestore(ctx, args)
	case "status":
		return auditStatus(ctx, args)
	default:
		return nil, fmt.Errorf("audit: unknown subcommand: %s", sub)
	}
}

// auditAdd creates a top-level audit.
var auditAddFlags = []sdk.Flag{
	{Name: "status", Short: "s", Type: "string"},
	{Name: "assign", Type: "string"},
	{Name: "file", Type: "string"},
	{Name: "version", Type: "int"},
}

func auditAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(auditAddFlags, args)
	if err != nil {
		return nil, fmt.Errorf("audit add: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("audit add: %w: target", sdk.ErrMissingArg)
	}

	target := positional[0]
	content := strings.Join(positional[1:], " ")

	// Content resolution: positional > --file > stdin.
	if content == "" {
		if f := flags.String("file"); f != "" {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("audit add: %w", err)
			}
			content = string(data)
		} else if len(ctx.Stdin) > 0 {
			content = string(ctx.Stdin)
		}
	}

	aud, err := ctx.Audits.Add(sdk.AuditOpts{
		Target:   target,
		Content:  content,
		Author:   ctx.Author,
		Assignee: flags.String("assign"),
		Status:   flags.String("status"),
		Version:  flags.Int("version"),
	})
	if err != nil {
		return nil, fmt.Errorf("audit add: %w", err)
	}

	text := fmt.Sprintf("Created audit %s on %s [%s]", aud.ID, aud.Target, aud.Status)
	return sdk.Result{Text: text, Data: aud}, nil
}

// auditReply responds to an existing audit thread.
var auditReplyFlags = []sdk.Flag{
	{Name: "status", Short: "s", Type: "string"},
	{Name: "assign", Type: "string"},
	{Name: "file", Type: "string"},
}

func auditReply(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(auditReplyFlags, args)
	if err != nil {
		return nil, fmt.Errorf("audit reply: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("audit reply: %w: audit ID", sdk.ErrMissingArg)
	}

	id := positional[0]
	content := strings.Join(positional[1:], " ")

	if content == "" {
		if f := flags.String("file"); f != "" {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("audit reply: %w", err)
			}
			content = string(data)
		} else if len(ctx.Stdin) > 0 {
			content = string(ctx.Stdin)
		}
	}

	aud, err := ctx.Audits.Reply(id, sdk.AuditOpts{
		Content:  content,
		Author:   ctx.Author,
		Assignee: flags.String("assign"),
		Status:   flags.String("status"),
	})
	if err != nil {
		return nil, fmt.Errorf("audit reply: %w", err)
	}

	text := fmt.Sprintf("Replied to %s [%s]", aud.ParentID, aud.Status)
	return sdk.Result{Text: text, Data: aud}, nil
}

// auditList lists audits with optional filters.
var auditListFlags = []sdk.Flag{
	{Name: "status", Short: "s", Type: "string"},
	{Name: "assign", Type: "string", Desc: "Filter by assigned person"},
	{Name: "by-author", Type: "string", Desc: "Filter by creator"},
	{Name: "pending", Type: "bool"},
	{Name: "since", Type: "string", Desc: "Show audits created after (e.g. 5m, 1h, RFC 3339)"},
}

func auditList(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(auditListFlags, args)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}

	since, err := sdk.ParseSince(flags.String("since"))
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	opts := sdk.AuditListOpts{
		ByAuthor: flags.String("by-author"),
		Assignee: flags.String("assign"),
		Status:   flags.String("status"),
		Pending:  flags.Bool("pending"),
		Since:    since,
	}
	if len(positional) > 0 {
		opts.Target = positional[0]
	}

	audits, err := ctx.Audits.List(opts)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}

	if len(audits) == 0 {
		return sdk.Result{Text: "No audits found.", Data: audits}, nil
	}

	var b strings.Builder
	for _, a := range audits {
		ts := time.UnixMilli(a.CreatedAt).Format("2006-01-02")
		fmt.Fprintf(&b, "%-14s  %-20s  %-12s  %s  %s\n",
			a.ID, a.Target, a.Status, ts, a.Author)
	}
	return sdk.Result{Text: b.String(), Data: audits}, nil
}

// auditShow displays a full audit thread.
func auditShow(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audit show: %w: audit ID", sdk.ErrMissingArg)
	}

	thread, err := ctx.Audits.Thread(args[0])
	if err != nil {
		return nil, fmt.Errorf("audit show: %w", err)
	}

	var b strings.Builder
	for i, a := range thread {
		ts := time.UnixMilli(a.CreatedAt).Format("2006-01-02 15:04")
		prefix := ""
		if i > 0 {
			prefix = "  └ "
		}
		fmt.Fprintf(&b, "%s%s  %s  [%s]  %s\n", prefix, a.Author, ts, a.Status, a.ID)
		content := a.Content
		if content == "" {
			content = "(no content)"
		}
		if i > 0 {
			// Indent reply content.
			for line := range strings.SplitSeq(content, "\n") {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		} else {
			fmt.Fprintf(&b, "%s\n", content)
		}
		if i < len(thread)-1 {
			b.WriteByte('\n')
		}
	}
	return sdk.Result{Text: b.String(), Data: thread}, nil
}

// auditResolve marks an audit as approved.
func auditResolve(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audit resolve: %w: audit ID", sdk.ErrMissingArg)
	}

	aud, err := ctx.Audits.Resolve(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("audit resolve: %w", err)
	}

	text := fmt.Sprintf("Resolved %s [approved]", aud.ParentID)
	return sdk.Result{Text: text, Data: aud}, nil
}

// auditRm soft-deletes an audit.
func auditRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audit rm: %w: audit ID", sdk.ErrMissingArg)
	}

	if err := ctx.Audits.Delete(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("audit rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Deleted audit %s", args[0])), nil
}

// auditRestore recovers a soft-deleted audit.
func auditRestore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audit restore: %w: audit ID", sdk.ErrMissingArg)
	}

	aud, err := ctx.Audits.Restore(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("audit restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored audit %s", aud.ID)), nil
}

var auditStatusFlags = []sdk.Flag{
	{Name: "since", Type: "string", Desc: "Show audits created after (e.g. 5m, 1h, RFC 3339)"},
}

// auditStatus shows the agent's inbox.
func auditStatus(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(auditStatusFlags, args)
	if err != nil {
		return nil, fmt.Errorf("audit status: %w", err)
	}
	since, err := sdk.ParseSince(flags.String("since"))
	if err != nil {
		return nil, fmt.Errorf("audit status: %w", err)
	}
	result, err := ctx.Audits.Status(ctx.Author, sdk.AuditStatusOpts{Since: since})
	if err != nil {
		return nil, fmt.Errorf("audit status: %w", err)
	}

	if result.Summary.Total == 0 {
		text := fmt.Sprintf("No pending audits for %s.", result.Author)
		return sdk.Result{Text: text, Data: result}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Pending audits for %s: %d", result.Author, result.Summary.Total)
	if result.Summary.NeedsWork > 0 {
		fmt.Fprintf(&b, " (%d needs-work)", result.Summary.NeedsWork)
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	for _, a := range result.Pending {
		ts := time.UnixMilli(a.CreatedAt).Format("2006-01-02")
		fmt.Fprintf(&b, "  %-14s  %-20s  %-12s  %s  %s\n",
			a.ID, a.Target, a.Status, ts, a.Author)
	}
	return sdk.Result{Text: b.String(), Data: result}, nil
}
