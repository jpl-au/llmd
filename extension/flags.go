// flags.go defines constants for all CLI flag names.
//
// Using constants instead of string literals prevents typos and enables
// compile-time checking when flag names are used in both Flags().Type()
// definitions and GetType() calls.
//
// Naming convention: Flag<PascalCaseName> where name matches the kebab-case
// CLI flag (e.g., "dry-run" -> FlagDryRun).

package extension

// Flag name constants for CLI commands.
// These are used with cobra's Flags().Type() and GetType() methods.
const (
	// Boolean flags

	FlagAll            = "all"                // Include all items (including deleted)
	FlagCount          = "count"              // Output count only
	FlagDeleted        = "deleted"            // Include/show deleted items
	FlagDiff           = "diff"               // Show diff output
	FlagDryRun         = "dry-run"            // Preview without making changes
	FlagFile           = "file"               // Treat path as filesystem file
	FlagFilesWithMatch = "files-with-matches" // Output matching file paths only
	FlagFlat           = "flat"               // Flatten directory structure
	FlagIgnoreCase     = "ignore-case"        // Case-insensitive matching
	FlagIncludeHidden  = "include-hidden"     // Include hidden files/directories
	FlagInPlace        = "in-place"           // Edit in place (required for sed)
	FlagInvertMatch    = "invert-match"       // Invert match selection
	FlagList           = "list"               // List mode
	FlagLocal          = "local"              // Use local scope (gitignored)
	FlagLong           = "long"               // Long format output
	FlagNumber         = "number"             // Number output lines
	FlagOrphan         = "orphan"             // Show orphaned items
	FlagPathsOnly      = "paths-only"         // Output paths only
	FlagRaw            = "raw"                // Raw output without formatting
	FlagRecursive      = "recursive"          // Recursive operation
	FlagReverse        = "reverse"            // Reverse sort order
	FlagShare          = "share"              // Mark as shared (committed)
	FlagTree           = "tree"               // Tree view output

	// String flags

	FlagLines     = "lines"      // Line range specification (e.g., "10:20")
	FlagNew       = "new"        // New text for replacement
	FlagOld       = "old"        // Old text to find
	FlagOlderThan = "older-than" // Duration threshold
	FlagPath      = "path"       // Path prefix filter
	FlagSort      = "sort"       // Sort field
	FlagTag       = "tag"        // Tag filter/value
	FlagTo        = "to"         // Target path prefix
	FlagVersions  = "versions"   // Version range (e.g., "3:5")

	// Integer flags

	FlagContext = "context" // Context lines around matches
	FlagLimit   = "limit"   // Limit number of results
	FlagVersion = "version" // Specific version number
)
