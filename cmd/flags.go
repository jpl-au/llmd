/*
Copyright © 2026 James Lawson (jpl-au) <hello@caelisco.net>
*/

// flags.go defines global CLI flags and accessors for shared state.
//
// Separated from root.go to isolate flag definitions from command logic.
// Extensions access these via exported accessor functions rather than
// directly accessing the variables.
//
// Design: Flags are defined as package-level variables and bound to the
// root command. Accessors are provided so extensions can read flag values
// without coupling to cobra internals. The JSON() helper simplifies output
// format detection across all commands.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/spf13/cobra"
)

var validOutputFormats = []string{"json"}

var (
	output  string
	author  string
	message string
	force   bool
	db      string
	dir     string
)

// out is the output writer for commands. Defaults to os.Stdout.
// Tests can replace this to capture output.
var out io.Writer = os.Stdout

// Exported accessors for extensions.
// Extensions use these to access shared CLI state.

// Out returns the output writer.
func Out() io.Writer { return out }

// Output returns the output format flag value.
func Output() string { return output }

// Author returns the author flag value.
func Author() string { return author }

// Message returns the message flag value.
func Message() string { return message }

// Force returns the force flag value.
func Force() bool { return force }

// DB returns the resolved database name.
// Priority: --db flag > LLMD_DB env var > empty (default).
func DB() string {
	if db != "" {
		return db
	}
	return os.Getenv("LLMD_DB")
}

// Dir returns the explicit database directory if set.
// Priority: --dir flag > LLMD_DIR env var > empty (use discovery).
func Dir() string {
	if dir != "" {
		return dir
	}
	return os.Getenv("LLMD_DIR")
}

// SetOut sets the output writer (for testing).
func SetOut(w io.Writer) { out = w }

// JSON returns true if JSON output is requested.
func JSON() bool { return output == "json" }

// PrintJSON marshals v to JSON and writes it to the output writer.
// Returns nil if output format is not JSON.
func PrintJSON(v any) error {
	if output != "json" {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Fprintln(out, string(b))
	return nil
}

// PrintJSONError prints an error in JSON format if output is JSON.
// Returns nil if error was printed (suppressing Cobra error), or the original error if not.
func PrintJSONError(err error) error {
	if output != "json" || err == nil {
		return err
	}
	// We ignore the error from PrintJSON here because if we can't print the error,
	// checking it is futile. We just return nil to suppress Cobra's duplicate printing.
	_ = PrintJSON(map[string]string{"error": err.Error()})
	return nil
}

// detectAuthor resolves the default author for version attribution.
// Returns empty string when config is missing or has no author set.
func detectAuthor() string {
	if cfg, err := config.Load(); err == nil && cfg.Author.Name != "" {
		return cfg.Author.Name
	}
	return ""
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "Output format: json")
	rootCmd.PersistentFlags().StringVarP(&author, "author", "a", "", "Version attribution")
	rootCmd.PersistentFlags().StringVarP(&message, "message", "m", "", "Version message")
	rootCmd.PersistentFlags().BoolVar(&force, "force", false, "Skip confirmations")
	rootCmd.PersistentFlags().StringVar(&db, "db", "", "Database name (e.g., docs for llmd-docs.db)")
	rootCmd.PersistentFlags().StringVar(&dir, "dir", "", "Database directory (skip discovery, use explicit path)")

	_ = rootCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return validOutputFormats, cobra.ShellCompDirectiveNoFileComp
	})
}
