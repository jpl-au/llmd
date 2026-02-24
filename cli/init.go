package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// initCmd creates a new llmd store. Uses ctx.DBPath if set,
// otherwise defaults to .llmd/llmd.db.
func initCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	store, err := llmd.Init(ctx.DBPath)
	if err != nil {
		return nil, err
	}
	path := store.Path()
	store.Close()
	return sdk.Text(fmt.Sprintf("Initialised llmd store at %s", path)), nil
}
