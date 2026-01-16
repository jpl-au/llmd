// Package sync provides the sync extension for llmd.
// It registers commands: import, export, sync.
//
// Note: This extension does not implement Initializable because the import
// command with --dry-run should work without a store. Commands create the
// service when needed, similar to the core extension.
package sync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/exporter"
	"github.com/jpl-au/llmd/internal/importer"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sync"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the sync extension.
// It does not implement Initializable because import --dry-run should
// work without a store.
type Extension struct{}

// Compile-time interface compliance. Catches missing methods at build time
// rather than runtime, making interface changes safer to refactor.
var (
	_ extension.Extension = (*Extension)(nil)
	_ extension.Storeless = (*Extension)(nil)
)

// Name returns "sync" - this extension handles filesystem synchronisation.
func (e *Extension) Name() string { return "sync" }

// Commands returns import, export, and sync commands for filesystem integration.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		newImportCmd(),
		newExportCmd(),
		newSyncCmd(),
	}
}

// MCPTools returns nil - MCP import/export tools are in internal/mcp.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}

// NoStoreCommands returns commands that manage their own service lifecycle.
// All sync commands need this because import --dry-run must work without a store,
// and export/sync similarly manage their own service instances.
func (e *Extension) NoStoreCommands() []string {
	return []string{"import", "export", "sync"}
}

// --- import command ---

func newImportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "import <filesystem-path>",
		Short: "Bulk import markdown files from filesystem",
		Long: `Bulk import markdown files from filesystem into the store.

Recursively scans for .md files and imports them.`,
		Args: cobra.ExactArgs(1),
		RunE: runImport,
	}
	c.Flags().StringP(extension.FlagTo, "t", "", "Target path prefix")
	c.Flags().BoolP(extension.FlagFlat, "F", false, "Flatten directory structure")
	c.Flags().BoolP(extension.FlagDryRun, "n", false, "Show what would be imported")
	c.Flags().BoolP(extension.FlagIncludeHidden, "H", false, "Include hidden files/dirs")
	return c
}

func runImport(c *cobra.Command, args []string) error {
	var ctx context.Context = c.Context()
	src := args[0]
	opts := importer.Options{
		Author: cmd.Author(),
		Msg:    cmd.Message(),
	}
	opts.Prefix, _ = c.Flags().GetString(extension.FlagTo)
	opts.Flat, _ = c.Flags().GetBool(extension.FlagFlat)
	opts.DryRun, _ = c.Flags().GetBool(extension.FlagDryRun)
	opts.Hidden, _ = c.Flags().GetBool(extension.FlagIncludeHidden)

	var svc *document.Service
	var err error
	if !opts.DryRun {
		svc, err = document.New(cmd.DB())
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("open store: %w", err))
		}
		defer svc.Close()
	}

	result, err := importer.Run(ctx, cmd.Out(), svc, src, opts)

	log.Event("sync:import", "import").
		Author(cmd.Author()).
		Detail("source", src).
		Detail("count", result.Imported).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("import %q: %w", src, err))
	}

	if len(result.Paths) == 0 {
		fmt.Fprintf(cmd.Out(), "No markdown files found in %q (expected .md files)\n", src)
		return nil
	}

	if !opts.DryRun {
		fmt.Fprintf(cmd.Out(), "\nImported %d file(s)\n", result.Imported)
	}
	return nil
}

// --- export command ---

func newExportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "export <doc-path> <filesystem-path>",
		Short: "Export documents from store to filesystem",
		Long: `Export documents from the store to filesystem.

Single document: destination can be a file path
Multiple documents (prefix): destination must be a directory`,
		Args: cobra.ExactArgs(2),
		RunE: runExport,
	}
	c.Flags().IntP(extension.FlagVersion, "v", 0, "Export specific version")
	c.Flags().StringP(extension.FlagKey, "k", "", "Export by version key (8-char identifier)")
	return c
}

func runExport(c *cobra.Command, args []string) error {
	ctx := c.Context()
	docPath, dest := args[0], args[1]
	svc, err := document.New(cmd.DB())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("open store: %w", err))
	}
	defer svc.Close()

	opts := exporter.Options{
		Force: cmd.Force(),
	}
	opts.Version, _ = c.Flags().GetInt(extension.FlagVersion)
	keyFlag, _ := c.Flags().GetString(extension.FlagKey)

	key := ""
	if keyFlag != "" {
		// Explicit key provided via --key flag
		doc, err := svc.ByKey(ctx, keyFlag)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("key %q: %w", keyFlag, err))
		}
		key = keyFlag
		docPath = doc.Path
		opts.Version = doc.Version
	} else if opts.Version == 0 {
		// No version specified - try to resolve as path or key
		doc, isKey, err := svc.Resolve(ctx, docPath, false)
		if err == nil && isKey {
			key = docPath
			docPath = doc.Path
			opts.Version = doc.Version
		}
		// If err or resolved as path, let exporter.Run handle it
	}

	result, err := exporter.Run(ctx, cmd.Out(), svc, docPath, dest, opts)

	logEvent := log.Event("sync:export", "export").
		Author(cmd.Author()).
		Path(docPath).
		Detail("dest", dest).
		Detail("count", result.Exported)
	if key != "" {
		logEvent.Detail("key", key)
	}
	logEvent.Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("export %q to %q: %w", docPath, dest, err))
	}

	if result.Exported > 1 {
		fmt.Fprintf(cmd.Out(), "\nExported %d file(s)\n", result.Exported)
	} else if key != "" && result.Exported == 1 {
		fmt.Fprintf(cmd.Out(), "(from key %s)\n", key)
	}
	return nil
}

// --- sync command ---

func newSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync filesystem changes back to database",
		Long: `Detect files that were modified directly in .llmd/files/ and import
those changes back into the database.

This is a recovery mechanism for when files are edited directly
(bypassing llmd commands).`,
		RunE: runSync,
	}
	c.Flags().BoolP(extension.FlagDryRun, "n", false, "Show what would be synced")
	return c
}

func runSync(c *cobra.Command, _ []string) error {
	ctx := c.Context()
	svc, err := document.New(cmd.DB())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("open store: %w", err))
	}
	defer svc.Close()

	dir := svc.FilesDir()
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(cmd.Out(), "No files directory found")
		return nil
	}

	docs, err := svc.List(ctx, "", false, false)
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("list documents: %w", err))
	}

	db := make(map[string]string, len(docs))
	for _, d := range docs {
		db[d.Path] = d.Content
	}

	opts := sync.Options{
		Author: cmd.Author(),
		Msg:    cmd.Message(),
	}
	opts.DryRun, _ = c.Flags().GetBool(extension.FlagDryRun)

	result, err := sync.Run(ctx, cmd.Out(), svc, dir, db, opts)

	log.Event("sync:sync", "sync").
		Author(cmd.Author()).
		Detail("added", result.Added).
		Detail("updated", result.Updated).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("sync: %w", err))
	}

	total := result.Updated + result.Added
	if total == 0 {
		fmt.Fprintln(cmd.Out(), "No changes detected")
		return nil
	}

	if !opts.DryRun {
		fmt.Fprintf(cmd.Out(), "\nSynced %d file(s)\n", total)
	}
	return nil
}
