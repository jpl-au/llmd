// Package settings keeps the settings of a store: values by key that
// hold for everyone who uses the store, such as the harness an agent
// runs under. Settings that differ from one computer to the next live
// in the configuration file instead (package config).
//
// Each Set appends a row, so a setting keeps its history, and the
// newest row for a key is its value.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/clock"
	"github.com/jpl-au/llmd/engine/events"
	"github.com/jpl-au/llmd/engine/lazy"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// Schema is the DDL for the settings table.
const Schema = `
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    author TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key, id);
`

// Settings reads and writes the settings of a store in the work
// database.
type Settings struct {
	dbFn func() *qwr.Manager
	db   *qwr.Manager
	bus  *events.Bus
	once lazy.Once
}

// New creates a Settings. The db function returns the work database
// manager, opening it on demand.
func New(db func() *qwr.Manager, bus *events.Bus) *Settings {
	return &Settings{dbFn: db, bus: bus}
}

// ensure opens the work database and creates the settings table if
// needed.
func (s *Settings) ensure() error {
	return s.once.Do(func() error {
		db := s.dbFn()
		if db == nil {
			return fmt.Errorf("work database not available")
		}
		if _, err := db.Query(Schema).Exec(); err != nil {
			return err
		}
		s.db = db
		return nil
	})
}

// Value returns the value of key. The second result is false when the
// key has never been set.
func (s *Settings) Value(ctx context.Context, key string) (string, bool, error) {
	if err := s.ensure(); err != nil {
		return "", false, err
	}
	row, err := s.db.Query(
		`SELECT value FROM settings WHERE key = ? ORDER BY id DESC LIMIT 1`, key,
	).WithContext(ctx).ReadRow()
	if err != nil {
		return "", false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	return value, true, nil
}

// Set stores value under key and emits a SettingChanged event.
func (s *Settings) Set(ctx context.Context, key, value, author string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	now := clock.Now()
	if _, err := s.db.Query(
		`INSERT INTO settings (key, value, author, created_at) VALUES (?, ?, ?, ?)`,
		key, value, author, now,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("writing setting %s: %w", key, err)
	}
	if s.bus != nil {
		s.bus.Emit(ctx, pkgevents.Event{
			Type:      pkgevents.SettingChanged,
			Key:       key,
			Author:    author,
			Timestamp: now,
			Metadata:  map[string]any{"value": value},
		})
	}
	return nil
}
