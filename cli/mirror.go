// mirror.go dumps all documents to the filesystem as .md files so editors
// and AI agents can reference them (e.g. @ mentions in Claude Code).
//
// Files are written to .llmd/mirror/ preserving the document path structure.
// This is a one-way push — the filesystem copy is disposable and regenerated
// from the store each time. Use import/export for bidirectional operations.
//
// Usage:
//
//	llmd mirror              Mirror all documents
//	llmd mirror <prefix>     Mirror documents under a prefix

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

const mirrorDir = ".llmd/mirror"

func mirror(_ sdk.Context, args []string) (sdk.Response, error) {
	var prefix string
	if len(args) > 0 {
		prefix = args[0]
	}

	r, err := sdk.Documents.Mirror(prefix, mirrorDir)
	if err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}

	var parts []string
	if r.Wrote > 0 {
		parts = append(parts, fmt.Sprintf("wrote %d", r.Wrote))
	}
	if r.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("unchanged %d", r.Skipped))
	}
	if r.Removed > 0 {
		parts = append(parts, fmt.Sprintf("removed %d stale", r.Removed))
	}
	if len(parts) == 0 {
		return sdk.Text("Nothing to mirror"), nil
	}

	return sdk.Text(fmt.Sprintf("Mirrored to %s/ (%s)", mirrorDir, strings.Join(parts, ", "))), nil
}
