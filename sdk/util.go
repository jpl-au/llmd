//go:build wasip1

package sdk

import "strings"

// SplitCommand splits a command string that may include subcommands.
//
// Useful for plugins that implement command hierarchies (e.g., "tag add",
// "tag remove"). The command string is split on whitespace.
//
// Example:
//
//	parts := sdk.SplitCommand("tag add")
//	// parts = ["tag", "add"]
func SplitCommand(cmd string) []string {
	return strings.Fields(cmd)
}
