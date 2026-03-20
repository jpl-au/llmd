package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var vacuumSpec = sdk.Command{
	Name: "vacuum", Desc: `Permanently purge soft-deleted documents and reclaim space

Removes all documents deleted with rm, along with their version
history, orphaned tags, and orphaned links. This cannot be undone.`, Usage: "vacuum",
}

// vacuumCmd permanently deletes all soft-deleted documents, tags, and
// links, then runs SQLite VACUUM to reclaim disk space. This is
// irreversible - soft-deleted documents cannot be restored after vacuum.
func vacuumCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	result, err := ctx.Documents.Vacuum()
	if err != nil {
		return nil, fmt.Errorf("vacuum: %w", err)
	}
	total := result.Documents + result.Tags + result.Links
	if total == 0 {
		return sdk.Text("Nothing to vacuum"), nil
	}
	return sdk.Text(fmt.Sprintf("Vacuumed %d documents, %d tags, %d links",
		result.Documents, result.Tags, result.Links)), nil
}
