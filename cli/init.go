package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var initSpec = sdk.Command{
	Name: "init", Desc: `Create a new document store in the current directory

Creates .llmd/llmd.db. Safe to run if a store already exists — it
will not overwrite existing data. Use --db to specify a custom path.`, Usage: "init",
}

// initCmd creates a new llmd store. Uses ctx.DBPath if set,
// otherwise defaults to .llmd/llmd.db.
func initCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	path, err := sdk.Init(ctx.DBPath)
	if err != nil {
		return nil, err
	}
	return sdk.Text(fmt.Sprintf("Initialised llmd store at %s", path)), nil
}
