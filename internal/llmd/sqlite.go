// sqlite.go creates the store's schema.

package llmd

import (
	"github.com/jpl-au/llmd/internal/sql"
)

// createSchema runs the embedded SQL files from internal/sql in
// alphabetical order. It runs on every Open and Init: every statement
// is CREATE IF NOT EXISTS or INSERT OR IGNORE, so it creates what a
// fresh store needs and does nothing to a store that has it. There is
// no upgrade path for an older schema; a store is always treated as a
// fresh install.
func (s *Store) createSchema() error {
	return sql.Exec(s.db)
}
