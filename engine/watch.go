// watch.go notices commits other processes make to the store.

package engine

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	pkgevents "github.com/jpl-au/llmd/pkg/events"

	_ "modernc.org/sqlite" // the driver qwr uses, named "sqlite"
)

// Watch polls llmd.db and work.db for commits made by other processes
// and calls fn with the file name of the database that changed, until
// ctx is done. Call it once per process.
//
// It reads SQLite's data_version on a connection of its own, which
// advances on every commit that connection did not make. Commits this
// process makes also raise an event on the bus, but only after the
// commit, so a poll can see one of ours before its event arrives. A
// change is therefore held for one interval and reported only if no
// event of ours arrived in the meantime; an outside commit landing in
// the same interval as one of ours is reported with the next outside
// commit. A work.db that appears while watching is reported as a
// change to it.
func (s *Store) Watch(ctx context.Context, interval time.Duration, fn func(db string)) error {
	if interval <= 0 {
		return errors.New("watch: interval must be positive")
	}
	var own atomic.Bool
	s.bus.Subscribe(&ownMarker{own: &own})

	dbs := []*dbWatch{{path: s.path}, {path: s.workPath}}
	defer func() {
		for _, w := range dbs {
			w.close()
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var held []string // changes seen last tick, not yet known to be outside
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		// Read the flag once per tick so changes of ours and outside
		// changes are judged together. An event of ours that arrived
		// since the last tick claims the changes held from it too.
		mine := own.Swap(false)
		if !mine {
			for _, db := range held {
				fn(db)
			}
		}
		held = held[:0]
		for _, w := range dbs {
			changed, err := w.poll(ctx)
			if err != nil {
				slog.Debug("watch: polling database", "path", w.path, "err", err)
				continue
			}
			if changed && !mine {
				held = append(held, filepath.Base(w.path))
			}
		}
	}
}

// ownMarker notes that this process changed the store. It implements
// events.Handler and never fails, so it cannot stop delivery.
type ownMarker struct{ own *atomic.Bool }

func (m *ownMarker) HandleEvent(context.Context, pkgevents.Event) error {
	m.own.Store(true)
	return nil
}

// dbWatch is one database under watch: a pinned connection and the
// data_version it last saw.
type dbWatch struct {
	path    string
	db      *sql.DB
	conn    *sql.Conn
	version int64
	absent  bool // the file was missing on an earlier poll
}

// poll reports whether the database has changed since the last poll.
// The first poll of an existing file only records a baseline. A file
// seen missing earlier and present now is a change.
func (w *dbWatch) poll(ctx context.Context) (bool, error) {
	if w.conn == nil {
		if _, err := os.Stat(w.path); err != nil {
			w.absent = true
			return false, nil
		}
		db, err := sql.Open("sqlite", w.path)
		if err != nil {
			return false, err
		}
		db.SetMaxOpenConns(1)
		conn, err := db.Conn(ctx)
		if err != nil {
			return false, errors.Join(err, db.Close())
		}
		w.db, w.conn = db, conn
		if err := w.read(ctx); err != nil {
			w.close()
			return false, err
		}
		return w.absent, nil
	}
	last := w.version
	if err := w.read(ctx); err != nil {
		return false, err
	}
	return w.version != last, nil
}

// read refreshes the data_version seen on the pinned connection.
func (w *dbWatch) read(ctx context.Context) error {
	return w.conn.QueryRowContext(ctx, "PRAGMA data_version").Scan(&w.version)
}

// close releases the connection, if one was opened.
func (w *dbWatch) close() {
	if w.conn != nil {
		if err := w.conn.Close(); err != nil {
			slog.Debug("watch: closing connection", "path", w.path, "err", err)
		}
		w.conn = nil
	}
	if w.db != nil {
		if err := w.db.Close(); err != nil {
			slog.Debug("watch: closing database", "path", w.path, "err", err)
		}
		w.db = nil
	}
}
