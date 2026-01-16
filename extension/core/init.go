// init.go implements the "llmd init" command for repository initialisation.
//
// Separated from extension.go to isolate init-specific logic. Init is special
// because it runs before a store exists and creates the initial database.
//
// Design: Init does NOT create config - that's managed separately via
// "llmd config". This follows git's model where init creates repository
// structure and config is separate. The --local flag controls whether the
// database is committed to git or gitignored.

package core

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/repo"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new LLMD store",
		Long: `Creates a .llmd/llmd.db database in the current directory.

Use --db to create additional databases:
  llmd init --db docs    # creates .llmd/llmd-docs.db

Use --dir to create in a different directory:
  llmd init --dir /path/to/project    # creates /path/to/project/.llmd/llmd.db

Use --local to exclude from git:
  llmd init --db notes --local    # creates llmd-notes.db, not committed

Note: init does not create config. Use "llmd config" to set up configuration.`,
		RunE: runInit,
	}
	c.Flags().BoolP(extension.FlagLocal, "l", false, "Mark database as local (gitignored)")
	return c
}

func runInit(c *cobra.Command, _ []string) error {
	local, _ := c.Flags().GetBool(extension.FlagLocal)
	db, dir := cmd.DB(), cmd.Dir()

	// Validate flag combinations.
	//
	// Why --local and --dir are incompatible: The --local flag adds the database
	// to the current project's .gitignore. When using --dir, you're creating a
	// database in an external directory - adding it to the current project's
	// gitignore makes no sense since the database isn't here. Users working with
	// external databases manage git exclusions in those projects directly.
	if local && dir != "" {
		return cmd.PrintJSONError(fmt.Errorf("cannot use --local with --dir: --local modifies the current project's .gitignore, but --dir creates the database elsewhere"))
	}

	err := document.Init(cmd.Force(), db, local, dir)

	log.Event("core:init", "init").
		Author(cmd.Author()).
		Detail("db", db).
		Detail("dir", dir).
		Detail("local", local).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("init: %w", err))
	}

	dbFile := repo.DBFileName(db)
	loc := ".llmd/" + dbFile
	if dir != "" {
		loc = dir + "/.llmd/" + dbFile
	}
	fmt.Fprintf(cmd.Out(), "Initialised LLMD store in %s\n", loc)
	return nil
}
