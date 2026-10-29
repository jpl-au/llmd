package cli

import (
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var initSpec = sdk.Command{
	Name: "init", Desc: `Create a new document store in the current directory

Creates .llmd/llmd.db. Safe to run if a store already exists - it
will not overwrite existing data. Use --db to specify a custom path.`, Usage: "init",
	MCP: true, MCPName: "llmd_init",
}

// initCmd creates a new llmd store. Uses ctx.DBPath if set,
// otherwise defaults to .llmd/llmd.db.
func initCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if _, _, err := sdk.ParseArgs(initSpec.Flags, args); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	path, err := ctx.Runtime.Init(ctx.DBPath)
	if errors.Is(err, sdk.ErrExists) {
		// Safe to run again: the store and its files are left as they
		// are, so a setup script can run init unconditionally.
		return sdk.Text(fmt.Sprintf("A store already exists at %s. Nothing changed.", path)), nil
	}
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Initialised llmd store at %s", path)), nil
}
