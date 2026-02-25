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

	"github.com/jpl-au/llmd/internal/llmd"
)

// Context provides extensions controlled access to llmd internals.
// Extensions receive this during initialisation to access shared resources.
type Context interface {
	// Store returns the document store for CRUD operations.
	Store() *llmd.Store

	// DB exposes the database for extensions needing custom tables.
	// Extensions should create their own tables, not modify core tables.
	DB() *sql.DB

	// Config returns user configuration for respecting user preferences.
	Config() map[string]string
}

// extContext implements [Context] by holding references to the store,
// database, and configuration. It is created by [NewContext] during host
// initialisation and passed to extensions that implement [Initializable].
// The struct is unexported to enforce construction via NewContext.
type extContext struct {
	store *llmd.Store
	db    *sql.DB
	cfg   map[string]string
}

// NewContext creates a new extension context with the given store,
// database connection, and user configuration. Called during host
// initialisation to build the context passed to [Initializable.Init].
func NewContext(store *llmd.Store, db *sql.DB, cfg map[string]string) Context {
	return &extContext{
		store: store,
		db:    db,
		cfg:   cfg,
	}
}

// Store returns the document store, the primary interface for document CRUD.
func (c *extContext) Store() *llmd.Store {
	return c.store
}

// DB returns the raw database connection for extensions needing custom tables.
func (c *extContext) DB() *sql.DB {
	return c.db
}

// Config returns the loaded user configuration for respecting preferences.
func (c *extContext) Config() map[string]string {
	return c.cfg
}
