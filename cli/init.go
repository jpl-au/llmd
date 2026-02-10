package cli

import (
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// initCmd creates a new llmd store in the current directory (.llmd/).
// It is a storeless command — it runs before any store exists.
func initCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	store, err := llmd.Init("")
	if err != nil {
		return nil, err
	}
	store.Close()
	return sdk.Text("Initialized llmd store in .llmd/"), nil
}
