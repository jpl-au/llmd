package llmd

import (
	"github.com/jpl-au/llmd/internal/sql"
)

// migrate applies the database schema.
func (s *Store) migrate() error {
	return sql.Exec(s.db)
}
