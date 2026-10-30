// Package sqlite opens the databases behind a store with one set of
// production settings, so every database llmd touches (llmd.db, work.db,
// and the databases package tests open) behaves the same way.
package sqlite

import (
	"time"

	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/checkpoint"
	"github.com/jpl-au/qwr/profile"
)

// Settings are the per-process SQLite settings a user may tune (the
// database.* config keys). A zero field means its default.
type Settings struct {
	// BusyTimeout is how long a write waits for another process's
	// write to finish before failing.
	BusyTimeout time.Duration
	// CacheMB is the most page cache, in MB, each connection may hold.
	CacheMB int
	// MMapMB is how much of each file, in MB, is memory-mapped. Nil
	// means the default; 0 turns memory-mapping off.
	MMapMB *int
	// Synchronous is how often writes are flushed to disk. Empty, or
	// a value it does not name, means SyncNormal.
	Synchronous Synchronous
}

// Synchronous is how often SQLite flushes writes to disk, named after
// SQLite's synchronous PRAGMA. A crash of llmd itself loses nothing
// with any value; the values differ in what a power failure or an
// operating system crash can do.
type Synchronous string

// The values of Synchronous.
const (
	// SyncFull flushes after every write, so a power failure loses
	// nothing. Every write waits for the disk.
	SyncFull Synchronous = "full"
	// SyncNormal flushes at checkpoints. A power failure can lose the
	// last writes but leaves the database intact.
	SyncNormal Synchronous = "normal"
	// SyncOff never flushes. A power failure can corrupt the
	// database, so it suits only databases that can be rebuilt, such
	// as the ones tests and CI create.
	SyncOff Synchronous = "off"
)

// Defaults for Settings.
const (
	DefaultBusyTimeout = 5 * time.Second
	DefaultCacheMB     = 64
	DefaultMMapMB      = 256
)

// Open opens or creates the database at path. qwr manages the SQLite
// pragmas via profiles (WAL, busy_timeout, foreign keys and so on),
// keeps separate reader and writer pools, and serialises writes. The
// WAL is truncated on checkpoint so the .db file stays self-contained
// for git.
//
// Page size and auto-vacuum are properties of the file, fixed by
// whichever connection creates it, so both pools are given the same
// values: 8 KB pages and no auto-vacuum (vacuum runs explicitly).
func Open(path string, s Settings) (*qwr.Manager, error) {
	busy := s.BusyTimeout
	if busy <= 0 {
		busy = DefaultBusyTimeout
	}
	cache := s.CacheMB
	if cache <= 0 {
		cache = DefaultCacheMB
	}
	mmap := DefaultMMapMB
	if s.MMapMB != nil && *s.MMapMB >= 0 {
		mmap = *s.MMapMB
	}
	tune := func(p *profile.Profile) *profile.Profile {
		p = p.
			WithPageSize(8192).
			WithAutoVacuum(profile.AutoVacuumNone).
			WithBusyTimeout(int(busy / time.Millisecond)).
			WithCacheSize(-cache * 1024). // negative: KiB, not pages
			WithMMapSize(int64(mmap) << 20)
		switch s.Synchronous {
		case SyncFull:
			p = p.WithSynchronous(profile.SyncFull)
		case SyncOff:
			p = p.WithSynchronous(profile.SyncOff)
		default:
			p = p.WithSynchronous(profile.SyncNormal)
		}
		return p
	}
	return qwr.New(path).
		Reader(tune(profile.ReadBalanced())).
		Writer(tune(profile.WriteBalanced())).
		Checkpoint(checkpoint.Truncate).
		Open()
}
