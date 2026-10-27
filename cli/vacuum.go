package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var vacuumSpec = sdk.Command{
	Name: "vacuum", Desc: `Permanently purge soft-deleted documents and reclaim space

Removes documents deleted with rm, along with their version history,
and the tags and links left orphaned by that removal, and queue
messages older than the queue.retention setting (default 30 days).
This cannot be undone, so preview first with --dry-run, and consider
--older-than to keep recent deletions restorable.`, Usage: "vacuum [--dry-run] [--older-than <age>]",
	NeedsAuthor: true,
	Flags: []sdk.Flag{
		{Name: "dry-run", Short: "n", Type: "bool", Desc: "Show what would be purged without changing anything"},
		{Name: "older-than", Type: "string", Desc: "Only purge items deleted more than this long ago (e.g. 30d, 12h) or before an RFC 3339 time"},
	},
}

// vacuumCmd purges soft-deleted documents, tags and links, then runs
// SQLite VACUUM to reclaim disk space. With --dry-run it only reports.
func vacuumCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(vacuumSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("vacuum: %w", err)
	}
	before, err := sdk.ParseSince(flags.String("older-than"))
	if err != nil {
		return nil, fmt.Errorf("vacuum --older-than: %w", err)
	}

	result, err := ctx.Documents.Vacuum(sdk.VacuumOpts{DryRun: flags.Bool("dry-run"), Before: before})
	if err != nil {
		return nil, fmt.Errorf("vacuum: %w", err)
	}

	total := result.Documents + result.Tags + result.Links + result.Messages
	if total == 0 {
		return sdk.Result{Text: "Nothing to vacuum", Data: result}, nil
	}

	verb := "Vacuumed"
	if result.DryRun {
		verb = "Would vacuum"
	}
	lines := []string{fmt.Sprintf("%s %d documents, %d tags, %d links, %d queue messages", verb, result.Documents, result.Tags, result.Links, result.Messages)}
	for _, p := range result.Paths {
		lines = append(lines, "  "+p)
	}
	return sdk.Result{Text: strings.Join(lines, "\n"), Data: result}, nil
}
