// Package link provides document relationship management. Links enable
// connecting related documents for navigation and dependency tracking.
// Registers commands: link, unlink.
package link

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the link extension.
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

// Name returns "link" - this extension manages document relationships.
func (e *Extension) Name() string { return "link" }

// Init connects to the shared service for document link operations.
func (e *Extension) Init(ctx extension.Context) error {
	e.svc = ctx.Service()
	return nil
}

// Commands returns link and unlink commands for relationship management.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		e.newLinkCmd(),
		e.newUnlinkCmd(),
	}
}

// MCPTools returns nil - MCP link tools are in internal/mcp.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}

// HandleEvent processes events from document operations to maintain link graph integrity.
//
// This method demonstrates how extensions can react to system events. The event system
// uses a fire-and-forget pattern: events are notifications, not approval requests.
// Handler errors are logged but don't block the originating operation.
//
// Why handle DocumentDeleteEvent here?
// When a document is deleted, any links pointing to or from it become "dangling" -
// they reference a path that no longer exists. Rather than leave these orphaned links
// in the database (which would confuse users and waste space), we proactively clean
// them up. This maintains referential integrity in the link graph automatically.
//
// Why not handle LinkEvent?
// LinkEvent is fired BY this extension's service calls (Link, UnlinkByID, UnlinkByTag).
// Handling our own events would be circular and potentially cause infinite loops.
// LinkEvent exists for other extensions that might want to react to link changes
// (e.g., a future search indexer or notification system).
func (e *Extension) HandleEvent(ctx extension.Context, evt extension.Event) error {
	switch ev := evt.(type) {
	case extension.DocumentDeleteEvent:
		return e.handleDocumentDelete(ctx, ev)
	}
	return nil
}

// handleDocumentDelete cleans up links when a document is soft-deleted.
//
// Referential integrity: Links form a graph where documents are nodes and links
// are edges. When a node is removed, its edges become meaningless - they point
// to/from nothing. We soft-delete these links to keep the graph consistent.
//
// Why soft-delete instead of hard-delete? The document itself is soft-deleted,
// meaning it can be restored via "llmd restore". If we hard-deleted the links,
// restoring the document would leave it disconnected from its former relationships.
// By soft-deleting links, a future enhancement could restore them alongside the
// document (though this isn't implemented yet).
//
// Performance consideration: For documents with many links, this performs one
// database operation (DeleteLinksForPath) rather than N individual deletions.
// The service method handles this efficiently with a single UPDATE statement.
func (e *Extension) handleDocumentDelete(extCtx extension.Context, ev extension.DocumentDeleteEvent) error {
	// DeleteLinksForPath soft-deletes all links where the document is either
	// the source (FromPath) or target (ToPath). This is idempotent - calling
	// it multiple times or on a document with no links is safe.
	// Note: Event handlers don't receive a context.Context from the caller,
	// so we use context.Background() here. This is acceptable for cleanup
	// operations that shouldn't be cancelled.
	if err := extCtx.Service().DeleteLinksForPath(context.Background(), ev.Path, store.NewLinkOptions()); err != nil {
		// Log and propagate the error so the user is aware of the failure.
		log.Event("link:cleanup", "event").
			Path(ev.Path).
			Detail("trigger", "document_delete").
			Write(err)
		return err
	}

	// Log successful cleanup for observability. This helps operators understand
	// what automated maintenance is happening and debug any issues.
	log.Event("link:cleanup", "event").
		Path(ev.Path).
		Detail("trigger", "document_delete").
		Detail("action", "links_removed").
		Write(nil)

	return nil
}

// --- link command ---

func (e *Extension) newLinkCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "link <document> [documents...]",
		Short: "Create links between documents",
		Long: `Create bidirectional links between documents.

  llmd link doc1 doc2              # link two documents
  llmd link doc1 doc2 doc3         # link doc1 to doc2 and doc3
  llmd link --tag depends-on a b   # link with a tag
  llmd link --list doc             # list links for a document
  llmd link --orphan               # find documents with no links`,
		Args: cobra.ArbitraryArgs,
		RunE: e.runLink,
	}
	c.Flags().StringP(extension.FlagTag, "t", "", "Link tag (optional categorisation)")
	c.Flags().BoolP(extension.FlagList, "l", false, "List links for a document")
	c.Flags().Bool(extension.FlagOrphan, false, "List documents with no links")
	return c
}

func (e *Extension) runLink(c *cobra.Command, args []string) error {
	ctx := c.Context()
	tag, _ := c.Flags().GetString(extension.FlagTag)
	list, _ := c.Flags().GetBool(extension.FlagList)
	orphan, _ := c.Flags().GetBool(extension.FlagOrphan)

	// --orphan: list unlinked documents
	if orphan {
		return e.listOrphans(ctx)
	}

	// --list: list links for a document
	if list {
		if len(args) == 0 {
			// List all links with tag
			if tag != "" {
				return e.listByTag(ctx, tag)
			}
			return cmd.PrintJSONError(fmt.Errorf("--list requires a document path or --tag"))
		}
		return e.listLinks(ctx, args[0], tag)
	}

	// Create links: need at least 2 documents
	if len(args) < 2 {
		return cmd.PrintJSONError(fmt.Errorf("link requires at least 2 documents"))
	}

	return e.createLinks(ctx, args[0], args[1:], tag)
}

// createLinks establishes bidirectional links between a source document and multiple targets.
// Arguments can be document paths or keys - both are resolved to paths before linking.
func (e *Extension) createLinks(ctx context.Context, from string, targets []string, tag string) error {
	// Resolve 'from' which could be a path or key
	fromDoc, _, err := e.svc.Resolve(ctx, from, false)
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("resolve %q: %w", from, err))
	}
	from = fromDoc.Path

	var ids []string
	var resolvedTargets []string
	for _, to := range targets {
		// Resolve 'to' which could be a path or key
		toDoc, _, err := e.svc.Resolve(ctx, to, false)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("resolve %q: %w", to, err))
		}
		to = toDoc.Path
		id, err := e.svc.Link(ctx, from, to, tag, store.NewLinkOptions())
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("link %q to %q: %w", from, to, err))
		}
		ids = append(ids, id)
		resolvedTargets = append(resolvedTargets, to)

		log.Event("link:create", "link").
			Author(cmd.Author()).
			Path(from).
			Detail("to", to).
			Detail("tag", tag).
			Detail("id", id).
			Write(nil)

		if !cmd.JSON() {
			if tag != "" {
				fmt.Fprintf(cmd.Out(), "%s  %s -> %s [%s]\n", id, from, to, tag)
			} else {
				fmt.Fprintf(cmd.Out(), "%s  %s -> %s\n", id, from, to)
			}
		}
	}

	if cmd.JSON() {
		return cmd.PrintJSON(map[string]any{
			"from":    from,
			"targets": resolvedTargets,
			"tag":     tag,
			"ids":     ids,
		})
	}
	return nil
}

// listLinks displays all links connected to a document, optionally filtered by tag.
// The path argument can be a document path or key.
func (e *Extension) listLinks(ctx context.Context, path, tag string) error {
	// Resolve path which could be a path or key
	doc, _, err := e.svc.Resolve(ctx, path, false)
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("resolve %q: %w", path, err))
	}
	path = doc.Path

	links, err := e.svc.ListLinks(ctx, path, tag, store.NewLinkOptions())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("list links for %q: %w", path, err))
	}

	log.Event("link:list", "list").
		Author(cmd.Author()).
		Path(path).
		Detail("tag", tag).
		Detail("count", len(links)).
		Write(nil)

	if cmd.JSON() {
		js := make([]store.LinkJSON, len(links))
		for i, l := range links {
			js[i] = l.ToJSON()
		}
		return cmd.PrintJSON(js)
	}

	for _, l := range links {
		other := l.ToPath
		if l.ToPath == path {
			other = l.FromPath
		}
		if l.Tag != "" {
			fmt.Fprintf(cmd.Out(), "%s  %s [%s]\n", l.ID, other, l.Tag)
		} else {
			fmt.Fprintf(cmd.Out(), "%s  %s\n", l.ID, other)
		}
	}
	return nil
}

// listByTag displays all links with a specific tag across the entire store.
func (e *Extension) listByTag(ctx context.Context, tag string) error {
	links, err := e.svc.ListLinksByTag(ctx, tag, store.NewLinkOptions())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("list links by tag %q: %w", tag, err))
	}

	log.Event("link:list_tag", "list").
		Author(cmd.Author()).
		Detail("tag", tag).
		Detail("count", len(links)).
		Write(nil)

	if cmd.JSON() {
		js := make([]store.LinkJSON, len(links))
		for i, l := range links {
			js[i] = l.ToJSON()
		}
		return cmd.PrintJSON(js)
	}

	for _, l := range links {
		fmt.Fprintf(cmd.Out(), "%s  %s -> %s\n", l.ID, l.FromPath, l.ToPath)
	}
	return nil
}

// listOrphans finds documents with no links, useful for discovering isolated content.
func (e *Extension) listOrphans(ctx context.Context) error {
	paths, err := e.svc.ListOrphanLinkPaths(ctx, store.NewLinkOptions())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("list orphans: %w", err))
	}

	log.Event("link:orphan", "list").
		Author(cmd.Author()).
		Detail("count", len(paths)).
		Write(nil)

	if cmd.JSON() {
		return cmd.PrintJSON(paths)
	}

	for _, p := range paths {
		fmt.Fprintln(cmd.Out(), p)
	}
	return nil
}

// --- unlink command ---

func (e *Extension) newUnlinkCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "unlink [id]",
		Short: "Remove a link",
		Long: `Remove a link by ID or remove all links with a tag.

  llmd unlink a1b2c3d4          # remove link by ID
  llmd unlink --tag depends-on  # remove all links with tag`,
		Args: cobra.MaximumNArgs(1),
		RunE: e.runUnlink,
	}
	c.Flags().StringP(extension.FlagTag, "t", "", "Remove all links with this tag")
	return c
}

func (e *Extension) runUnlink(c *cobra.Command, args []string) error {
	ctx := c.Context()
	tag, _ := c.Flags().GetString(extension.FlagTag)

	// Remove by tag
	if tag != "" {
		n, err := e.svc.UnlinkByTag(ctx, tag, store.NewLinkOptions())
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("unlink by tag %q: %w", tag, err))
		}

		log.Event("link:remove_tag", "unlink").
			Author(cmd.Author()).
			Detail("tag", tag).
			Detail("count", n).
			Write(nil)

		if cmd.JSON() {
			return cmd.PrintJSON(map[string]any{
				"tag":   tag,
				"count": n,
			})
		}

		fmt.Fprintf(cmd.Out(), "unlinked %d link(s) with tag %q\n", n, tag)
		return nil
	}

	// Remove by ID
	if len(args) != 1 {
		return cmd.PrintJSONError(fmt.Errorf("unlink requires a link ID or --tag"))
	}

	id := args[0]
	if err := e.svc.UnlinkByID(ctx, id); err != nil {
		return cmd.PrintJSONError(fmt.Errorf("unlink %q: %w", id, err))
	}

	log.Event("link:remove", "unlink").
		Author(cmd.Author()).
		Detail("id", id).
		Write(nil)

	if cmd.JSON() {
		return cmd.PrintJSON(map[string]any{
			"id": id,
		})
	}

	fmt.Fprintf(cmd.Out(), "unlinked %s\n", id)
	return nil
}
