//go:build telemetry

package telemetry

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Enabled reports whether telemetry is compiled into this build.
const Enabled = true

var (
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
)

// Init opens the telemetry log file in the .llmd directory. If the
// directory does not exist (no store initialised), falls back to the
// current working directory. Telemetry is silently disabled if the
// file cannot be opened - diagnostic logging must never break normal
// operation.
func Init() {
	path := filepath.Join(".llmd", "telemetry.jsonl")
	if _, err := os.Stat(".llmd"); os.IsNotExist(err) {
		path = "telemetry.jsonl"
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Debug("telemetry disabled: cannot open log", "path", path, "err", err)
		return
	}
	mu.Lock()
	file = f
	enc = json.NewEncoder(f)
	mu.Unlock()
	slog.Debug("telemetry enabled", "path", path)
}

// Close flushes and closes the telemetry log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
		enc = nil
	}
}

// Emit writes a single entry to the telemetry log. The timestamp is
// set automatically. Safe for concurrent use. If telemetry was not
// successfully initialised, the call is a no-op.
func Emit(e Entry) {
	mu.Lock()
	defer mu.Unlock()
	if enc == nil {
		return
	}
	e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if err := enc.Encode(e); err != nil {
		slog.Debug("telemetry write failed", "err", err)
	}
}
