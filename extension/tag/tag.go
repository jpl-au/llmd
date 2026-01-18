// Package tag provides the tag extension for llmd.
// It registers commands: tag (with subcommands add, rm, ls).
package tag

import (
	"fmt"
	"io"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/tag"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the tag extension.
type Extension struct {
	svc service.Service
}

// Compile-time interface compliance. Catches missing methods at build time
// rather than runtime, making interface changes safer to refactor.
var (
	_ extension.Extension     = (*Extension)(nil)
	_ extension.Initializable = (*Extension)(nil)
	_ extension.EventHandler  = (*Extension)(nil)
)

// Name returns "tag" - this extension provides document tagging commands.
func (e *Extension) Name() string { return "tag" }

// Init receives the shared service from the extension context.
func (e *Extension) Init(ctx extension.Context) error {
	e.svc = ctx.Service()
	return nil
}

// Commands returns the tag command with its subcommands (add, rm, ls).
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		e.newTagCmd(),
	}
}

// MCPTools returns nil - MCP tagging tools are in internal/mcp.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}

// HandleEvent processes document events for tag-related maintenance.
//
// This method demonstrates a second pattern for event handling, complementing
// the link extension's approach. While the link extension uses events for
// cleanup (deleting links when documents are deleted), this extension uses
// events for observability and potential future enhancements.
//
// Why handle DocumentWriteEvent?
// Document writes are the most common mutation in the system. By logging these
// events, we create an audit trail that can be useful for debugging, compliance,
// or understanding system behaviour. In the future, this hook could be extended
// to implement auto-tagging based on document content or path patterns.
//
// Why NOT handle TagEvent?
// TagEvent is fired by this extension's own service calls (Tag, Untag).
// Handling our own events would create unnecessary coupling and potential for
// confusion. TagEvent exists for other extensions that might want to react to
// tag changes (e.g., a search indexer that needs to update tag-based indices).
//
// Design note: This handler is intentionally lightweight. Heavy processing in
// event handlers can slow down the primary operation. For expensive operations,
// consider queueing work for background processing instead.
func (e *Extension) HandleEvent(ctx extension.Context, evt extension.Event) error { //nolint:revive // ctx for future use
	switch ev := evt.(type) {
	case extension.DocumentWriteEvent:
		return e.handleDocumentWrite(ev)
	case extension.DocumentDeleteEvent:
		return e.handleDocumentDelete(ev)
	}
	return nil
}

// handleDocumentWrite logs document write events for observability.
//
// This demonstrates how extensions can observe system activity without
// modifying behaviour. The log entries created here appear in the event
// log (viewable via "llmd db events") and can be used for:
// - Debugging: Understanding what operations occurred and when
// - Auditing: Tracking who modified which documents
// - Analytics: Measuring system activity patterns
//
// Future enhancement: This hook could implement auto-tagging rules, such as:
// - Tag documents in "drafts/" prefix with "draft"
// - Tag documents containing "TODO" with "needs-review"
// - Apply tags based on frontmatter metadata
func (e *Extension) handleDocumentWrite(ev extension.DocumentWriteEvent) error {
	// Log the write event for observability. This creates a record in the
	// event log that can be queried later for debugging or auditing.
	log.Event("tag:observed_write", "event").
		Path(ev.Path).
		Detail("version", ev.Version).
		Detail("author", ev.Author).
		Write(nil)

	return nil
}

// handleDocumentDelete logs document deletion events.
//
// Unlike the link extension which actively cleans up data on delete, the tag
// extension only logs the event. This is because:
// 1. Tags are stored per-document-path in the tags table
// 2. When a document is soft-deleted, its tags remain (enabling restore)
// 3. When vacuum permanently removes a document, tags are cleaned up then
//
// This handler exists primarily for observability and to demonstrate that
// multiple extensions can handle the same event type independently.
func (e *Extension) handleDocumentDelete(ev extension.DocumentDeleteEvent) error {
	log.Event("tag:observed_delete", "event").
		Path(ev.Path).
		Detail("reason", "document_deleted").
		Write(nil)

	return nil
}

// --- tag command with subcommands ---

func (e *Extension) newTagCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tag",
		Short: "Manage document tags",
		Long:  `Add, remove, and list tags for documents.`,
	}
	c.AddCommand(e.newTagAddCmd())
	c.AddCommand(e.newTagRmCmd())
	c.AddCommand(e.newTagLsCmd())
	return c
}

func (e *Extension) newTagAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path|key> <tag>",
		Short: "Add a tag to a document",
		Args:  cobra.ExactArgs(2),
		RunE:  e.runTagAdd,
	}
}

func (e *Extension) newTagRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <path|key> <tag>",
		Short: "Remove a tag from a document",
		Args:  cobra.ExactArgs(2),
		RunE:  e.runTagRm,
	}
}

func (e *Extension) newTagLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [path|key]",
		Short: "List tags for a document (or all tags if path omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  e.runTagLs,
	}
}

func (e *Extension) runTagAdd(c *cobra.Command, args []string) error {
	ctx := c.Context()
	path, t := args[0], args[1]
	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	l := log.Event("tag:add", "tag").
		Author(cmd.Author()).
		Path(path).
		Detail("tag", t)

	result, err := tag.Add(ctx, w, e.svc, path, t)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(fmt.Errorf("tag add %q %q: %w", path, t, err))
	}

	l.Resolved(result.Path).Write(nil)

	return cmd.PrintJSON(result)
}

func (e *Extension) runTagRm(c *cobra.Command, args []string) error {
	ctx := c.Context()
	path, t := args[0], args[1]
	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	l := log.Event("tag:rm", "untag").
		Author(cmd.Author()).
		Path(path).
		Detail("tag", t)

	result, err := tag.Remove(ctx, w, e.svc, path, t)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(fmt.Errorf("tag rm %q %q: %w", path, t, err))
	}

	l.Resolved(result.Path).Write(nil)

	return cmd.PrintJSON(result)
}

func (e *Extension) runTagLs(c *cobra.Command, args []string) error {
	ctx := c.Context()
	path := ""
	if len(args) > 0 {
		path = args[0]
	}

	w := cmd.Out()
	if cmd.JSON() {
		w = io.Discard
	}

	l := log.Event("tag:ls", "list_tags").
		Author(cmd.Author()).
		Path(path)

	result, err := tag.List(ctx, w, e.svc, path)
	if err != nil {
		l.Write(err)
		return cmd.PrintJSONError(fmt.Errorf("tag ls %q: %w", path, err))
	}

	l.Resolved(result.Path).
		Detail("count", len(result.Tags)).
		Write(nil)

	return cmd.PrintJSON(result)
}
