/*
Copyright © 2026 James Lawson (jpl-au) <hello@caelisco.net>
*/

// root.go defines the root command and CLI execution entry point.
//
// Separated from init_extensions.go to isolate cobra setup from extension
// initialisation logic.
//
// Design: PersistentPreRunE handles store initialisation lazily - only
// commands that need the store trigger extension init. This enables bootstrap
// commands (init, guide, config) to work without a store existing. The
// noStoreCommands map controls which commands skip initialisation.

package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "llmd",
	Short: "Versioned markdown document store for LLM workflows",
	Long:  `A versioned document store with filesystem-like commands (ls, cat, rm, mv), full-text search, and LLM integration.`,
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if output != "" && !slices.Contains(validOutputFormats, output) {
			return fmt.Errorf("invalid output format: %s (valid: %v)", output, validOutputFormats)
		}

		// Detect author if not explicitly set
		if author == "" {
			author = detectAuthor()
		}

		// Check if command requires author and none is configured
		cmdName := topLevelCmdName(cmd)
		if authorRequiredCommands[cmdName] && author == "" {
			return fmt.Errorf("author not configured (checked .llmd/config.yaml and ~/.llmd/config.yaml)\n\nRun: llmd config author.name \"Your Name\"\n\nSee 'llmd guide config' for local vs global options.")
		}

		// Initialise extensions for commands that need the store
		if !noStoreCommands[cmdName] {
			if err := initExtensions(); err != nil {
				if JSON() {
					_ = PrintJSON(map[string]string{"error": err.Error()})
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
				}
				return fmt.Errorf("initialise extensions: %w", err)
			}
		}

		return nil
	},
}

// topLevelCmdName returns the name of the top-level command (direct child of root).
// For "llmd cat docs/readme", returns "cat".
// For "llmd tag add path tag", returns "tag".
func topLevelCmdName(cmd *cobra.Command) string {
	// Walk up until we find a command whose parent has no parent (the root)
	for cmd.HasParent() && cmd.Parent().HasParent() {
		cmd = cmd.Parent()
	}
	return cmd.Name()
}

// Execute runs the root command and handles process lifecycle.
// Opens audit logging, registers extensions, executes the command, and ensures
// proper cleanup of the document service before exit. Exit code 1 indicates error.
func Execute() {
	// Initialise audit logger (warn if it fails, but continue)
	if err := log.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log unavailable: %v\n", err)
	}
	defer log.Close()

	registerExtensions()
	err := rootCmd.Execute()

	// Close the service if it was created
	if extService != nil {
		if closeErr := extService.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing service: %v\n", closeErr)
		}
	}

	if err != nil {
		os.Exit(1)
	}
}

// RootCmd returns the root command for testing and extension access.
func RootCmd() *cobra.Command {
	return rootCmd
}
