// context.go defines the Context interface for extension access to llmd internals.
//
// Separated from extension.go to isolate dependency injection concerns.
// The Context provides a controlled surface area for extensions - they can
// access what they need without reaching into arbitrary internals.
//
// Design: Context uses an interface to enable testing with mock implementations.
// Extensions receive Context during Init(), not at construction, to support
// the two-phase initialization pattern where extensions register before
// the service is available.

package extension

import (
	"database/sql"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/service"
)

// Context provides extensions controlled access to llmd internals.
// Extensions receive this during initialisation to access shared resources.
type Context interface {
	// Service returns the document service for CRUD operations.
	Service() service.Service

	// DB exposes the database for extensions needing custom tables.
	// Extensions should create their own tables, not modify core tables.
	DB() *sql.DB

	// Config returns user configuration for respecting user preferences.
	Config() *config.Config
}

// extContext implements Context.
type extContext struct {
	svc service.Service
	db  *sql.DB
	cfg *config.Config
}

// NewContext creates a new extension context.
func NewContext(svc service.Service, db *sql.DB, cfg *config.Config) Context {
	return &extContext{
		svc: svc,
		db:  db,
		cfg: cfg,
	}
}

// Service returns the document service, the primary interface for document CRUD.
func (c *extContext) Service() service.Service {
	return c.svc
}

// DB returns the raw database connection for extensions needing custom tables.
func (c *extContext) DB() *sql.DB {
	return c.db
}

// Config returns the loaded user configuration for respecting preferences.
func (c *extContext) Config() *config.Config {
	return c.cfg
}
