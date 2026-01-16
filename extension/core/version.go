// version.go implements the version command.

package core

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Print detailed version information including build date, git commit, Go version, and platform.`,
		Run: func(_ *cobra.Command, _ []string) {
			info := version.Get()
			if cmd.JSON() {
				_ = cmd.PrintJSON(info)
				return
			}
			fmt.Print(info.String())
		},
	}
}
