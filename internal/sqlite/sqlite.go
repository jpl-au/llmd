// Package sqlite opens the databases behind a store with one set of
// production settings, so every database llmd touches (llmd.db, work.db,
// and the databases package tests open) behaves the same way.
package sqlite

import (
	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/checkpoint"
	"github.com/jpl-au/qwr/profile"
)

// Open opens or creates the database at path. qwr manages the SQLite
// pragmas via profiles (WAL, busy_timeout, and so on), keeps separate
// reader and writer pools, and serialises writes. Foreign keys are not
// part of qwr's balanced profiles, so they are enabled here. The WAL is
// truncated on checkpoint so the .db file stays self-contained for git.
func Open(path string) (*qwr.Manager, error) {
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	return qwr.New(path).
		Reader(rp).
		Writer(wp).
		Checkpoint(checkpoint.Truncate).
		Open()
}
