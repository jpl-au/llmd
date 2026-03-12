// mirror.go syncs documents between the store and filesystem.
//
// Mirror maintains a filesystem copy of store documents so editors and
// AI agents can reference them (e.g. @ mentions in Claude Code). Files
// are written to .llmd/<dbname>/ preserving the document path structure.
//
// Usage:
//
//	llmd mirror [path]          Pull documents to filesystem
//	llmd mirror pull [path]     Pull documents to filesystem
//	llmd mirror push [path]     Push filesystem changes back to store

package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var mirrorSpec = sdk.Command{
	Name: "mirror", Desc: `Sync documents between the store and a local directory

Pull writes store documents to .llmd/<dbname>/ as .md files. Push
imports filesystem changes back into the store. Useful for editor
integration and AI agent access (e.g. @ mentions in Claude Code).`, Usage: "mirror [pull|push] [path]",
}

func mirror(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) > 0 {
		switch args[0] {
		case "pull":
			return mirrorPull(ctx, args[1:])
		case "push":
			return mirrorPush(ctx, args[1:])
		}
	}

	// No subcommand — pull is the default.
	return mirrorPull(ctx, args)
}

func mirrorPull(ctx sdk.Context, args []string) (sdk.Response, error) {
	var prefix string
	if len(args) > 0 {
		prefix = args[0]
	}

	dir := ctx.Mirror.Directory()
	r, err := ctx.Mirror.Pull(prefix, dir)
	if err != nil {
		return nil, fmt.Errorf("mirror pull: %w", err)
	}

	// Ensure the mirror directory is in .llmd/.gitignore.
	if err := ctx.Config.AddIgnore(filepath.Base(dir) + "/"); err != nil {
		return nil, fmt.Errorf("updating gitignore: %w", err)
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

	return sdk.Text(fmt.Sprintf("Pulled to %s/ (%s)", dir, strings.Join(parts, ", "))), nil
}

func mirrorPush(ctx sdk.Context, args []string) (sdk.Response, error) {
	if ctx.Author == "" {
		return nil, fmt.Errorf("mirror push: %w: author not configured", sdk.ErrMissingArg)
	}

	var prefix string
	if len(args) > 0 {
		prefix = args[0]
	}

	dir := ctx.Mirror.Directory()
	r, err := ctx.Mirror.Push(dir, sdk.PushOpts{
		Prefix: prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("mirror push: %w", err)
	}

	var parts []string
	if len(r.Created) > 0 {
		parts = append(parts, fmt.Sprintf("created %d", len(r.Created)))
	}
	if len(r.Updated) > 0 {
		parts = append(parts, fmt.Sprintf("updated %d", len(r.Updated)))
	}
	if len(r.Skipped) > 0 {
		parts = append(parts, fmt.Sprintf("unchanged %d", len(r.Skipped)))
	}
	if len(parts) == 0 {
		return sdk.Text("Nothing to push"), nil
	}

	return sdk.Text(fmt.Sprintf("Pushed from %s/ (%s)", dir, strings.Join(parts, ", "))), nil
}
