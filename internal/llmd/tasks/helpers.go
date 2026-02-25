// helpers.go contains small utility functions used across task operations.

package tasks

import (
	"database/sql"
	"strings"
)

// addFlag appends a flag to the comma-separated flags string. Returns
// the original string unchanged if the flag is already present.
func addFlag(flags, flag string) string {
	if flags == "" {
		return flag
	}
	for f := range strings.SplitSeq(flags, ",") {
		if f == flag {
			return flags // Already set
		}
	}
	return flags + "," + flag
}

// removeFlag removes a flag from the comma-separated flags string.
// Returns the remaining flags joined by commas, or empty string if
// no flags remain.
func removeFlag(flags, flag string) string {
	if flags == "" {
		return ""
	}
	var result []string
	for f := range strings.SplitSeq(flags, ",") {
		if f != flag {
			result = append(result, f)
		}
	}
	return strings.Join(result, ",")
}

// nullStr converts a Go string to sql.NullString. Empty strings become
// NULL in the database; non-empty strings become valid values.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
