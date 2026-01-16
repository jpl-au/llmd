// vacuum.go implements the "llmd vacuum" command for permanent deletion.
//
// Separated from extension.go because vacuum is destructive and requires
// special handling including confirmation prompts and dry-run support.
//
// Design: Vacuum is a NoStoreCommand to support --dry-run mode which needs
// to work even when the database might be in an unusual state. It manages
// its own service lifecycle to ensure proper cleanup after potentially
// long-running operations.

package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/duration"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/vacuum"
	"github.com/spf13/cobra"
)

func newVacuumCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "vacuum",
		Short: "Permanently delete soft-deleted documents",
		Long: `Permanently delete soft-deleted documents.

This is irreversible. Use --force to skip confirmation.

Duration formats: 7d (days), 4w (weeks), 3m (months)`,
		RunE: runVacuum,
	}
	c.Flags().String(extension.FlagOlderThan, "", "Only purge deletions older than duration (e.g., 7d, 4w, 3m)")
	c.Flags().StringP(extension.FlagPath, "p", "", "Only purge specific path prefix")
	c.Flags().BoolP(extension.FlagDryRun, "n", false, "Show what would be deleted")
	return c
}

func runVacuum(c *cobra.Command, _ []string) error {
	var ctx context.Context = c.Context()
	svc, err := document.New(cmd.DB())
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("open store: %w", err))
	}
	defer svc.Close()

	olderThan, _ := c.Flags().GetString(extension.FlagOlderThan)
	prefix, _ := c.Flags().GetString(extension.FlagPath)
	dryRun, _ := c.Flags().GetBool(extension.FlagDryRun)

	var opts vacuum.Options
	opts.Prefix = prefix
	opts.DryRun = dryRun

	if olderThan != "" {
		d, err := duration.Parse(olderThan)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("parse duration %q: %w", olderThan, err))
		}
		opts.OlderThan = &d
	}

	if dryRun {
		result, err := vacuum.Run(ctx, cmd.Out(), svc, opts)

		log.Event("core:vacuum", "vacuum").
			Author(cmd.Author()).
			Path(prefix).
			Detail("dry_run", true).
			Detail("count", result.Deleted).
			Write(err)

		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("vacuum dry run: %w", err))
		}
		return nil
	}

	if !cmd.Force() {
		fmt.Fprint(cmd.Out(), "Permanently delete soft-deleted documents? This cannot be undone. [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("reading confirmation: %w", err))
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.Out(), "Cancelled")
			return nil
		}
	}

	result, err := vacuum.Run(ctx, cmd.Out(), svc, opts)

	log.Event("core:vacuum", "vacuum").
		Author(cmd.Author()).
		Path(prefix).
		Detail("count", result.Deleted).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("vacuum: %w", err))
	}

	// Vacuum extension tables (extensions with custom tables implement Vacuumable)
	cfg, err := config.Load()
	if err != nil {
		return cmd.PrintJSONError(err)
	}
	extCtx := extension.NewContext(svc, svc.DB(), cfg)
	for _, ext := range extension.All() {
		if v, ok := ext.(extension.Vacuumable); ok {
			count, err := v.Vacuum(extCtx, opts.OlderThan)
			if err != nil {
				return cmd.PrintJSONError(fmt.Errorf("vacuum extension %s: %w", ext.Name(), err))
			}
			if count > 0 {
				fmt.Fprintf(cmd.Out(), "Vacuumed %d row(s) from %s\n", count, ext.Name())
			}
		}
	}

	return nil
}
