// db.go implements the "llmd db" command for database management.
//
// Separated from extension.go to isolate multi-database management logic
// including local/shared status toggling via gitignore manipulation.
//
// Design: DB is a NoStoreCommand because it manages database metadata
// (gitignore entries) without needing to open the databases themselves.
// This allows managing databases that might be locked or corrupted.

package core

import (
	"fmt"
	"path/filepath"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/repo"
	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "db [name]",
		Short: "List or manage databases",
		Long: `List databases or change their local/shared status.

  llmd db                    # list all databases
  llmd db --local            # mark default database as local
  llmd db notes --local      # mark notes database as local
  llmd db notes --share      # mark as shared
  llmd db --dir /path        # list databases in external directory

Local databases are not committed. Shared databases are.
If no name is given with --local or --share, operates on the default database.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runDB,
	}
	c.Flags().BoolP(extension.FlagLocal, "l", false, "Mark database as local")
	c.Flags().BoolP(extension.FlagShare, "s", false, "Mark database as shared")
	c.MarkFlagsMutuallyExclusive(extension.FlagLocal, extension.FlagShare)
	return c
}

func runDB(c *cobra.Command, args []string) error {
	local, _ := c.Flags().GetBool(extension.FlagLocal)
	share, _ := c.Flags().GetBool(extension.FlagShare)

	// Get --dir flag for explicit directory targeting.
	//
	// Why pass dir through: The db command manages gitignore entries in the
	// .llmd directory. Without --dir, it discovers the nearest .llmd directory
	// by walking up from the current directory. With --dir, it uses that path
	// directly. This allows managing databases in external projects.
	dir := cmd.Dir()

	// Convert --dir to the .llmd subdirectory path if provided.
	// repo functions expect the .llmd directory path, not the project root.
	llmdDir := ""
	if dir != "" {
		llmdDir = filepath.Join(dir, repo.Dir)
	}

	// No args and no flags: list databases
	if len(args) == 0 && !local && !share {
		err := listDBs(llmdDir)

		log.Event("core:db", "list").
			Author(cmd.Author()).
			Detail("dir", dir).
			Write(err)

		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("db list: %w", err))
		}
		return nil
	}

	// Get database name - empty string means default database
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	// Modify database status
	if local {
		err := repo.IgnoreDB(name, llmdDir)

		log.Event("core:db", "ignore").
			Author(cmd.Author()).
			Detail("db", name).
			Detail("dir", dir).
			Write(err)

		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("db ignore %q: %w", name, err))
		}
		fmt.Fprintf(cmd.Out(), "%s marked as local\n", repo.DBFileName(name))
		return nil
	}

	if share {
		err := repo.UnignoreDB(name, llmdDir)

		log.Event("core:db", "unignore").
			Author(cmd.Author()).
			Detail("db", name).
			Detail("dir", dir).
			Write(err)

		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("db unignore %q: %w", name, err))
		}
		fmt.Fprintf(cmd.Out(), "%s marked as shared\n", repo.DBFileName(name))
		return nil
	}

	// No flags with name: show status of that database
	ignored, err := repo.IsIgnored(name, llmdDir)

	log.Event("core:db", "status").
		Author(cmd.Author()).
		Detail("db", name).
		Detail("dir", dir).
		Write(err)

	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("db status %q: %w", name, err))
	}
	status := "shared"
	if ignored {
		status = "local"
	}
	fmt.Fprintf(cmd.Out(), "%s: %s\n", repo.DBFileName(name), status)
	return nil
}

// listDBs displays all databases in the target directory with their status.
// Each database shows as "shared" (committed) or "local" (gitignored).
func listDBs(dir string) error {
	dbs, err := repo.ListDBs(dir)
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("list databases: %w", err))
	}

	if len(dbs) == 0 {
		fmt.Fprintln(cmd.Out(), "No databases found")
		return nil
	}

	for _, db := range dbs {
		status := "shared"
		if db.Local {
			status = "local"
		}
		fmt.Fprintf(cmd.Out(), "%s  %s\n", db.File, status)
	}
	return nil
}
