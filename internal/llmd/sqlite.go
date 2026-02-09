package llmd

import (
	"github.com/jpl-au/llmd/internal/sql"
)

// migrate applies the database schema by executing all embedded SQL
// files from internal/sql in alphabetical order. This runs on every
// Open/Init — all schema statements use CREATE IF NOT EXISTS and
// INSERT OR IGNORE, making migration idempotent and safe to re-run.
func (s *Store) migrate() error {
	return sql.Exec(s.db)
}
