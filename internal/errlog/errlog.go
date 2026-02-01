// Package errlog provides error logging to the global llmd log directory.
//
// Errors are logged in JSONL format to ~/.llmd/logs/errors.jsonl (Unix) or
// %APPDATA%\llmd\logs\errors.jsonl (Windows). Each line is a JSON object with
// timestamp and error message.
//
// This package is used to capture full error details (including wazero stack
// traces) for debugging, while allowing the CLI to show clean messages to users.
package errlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	once    sync.Once
	logFile *os.File
)

// Log writes an error message to the global error log.
//
// The message is appended as a JSONL entry with a UTC timestamp.
// Logging failures are silently ignored - we don't want logging problems
// to cause user-facing errors.
func Log(msg string) {
	once.Do(openLogFile)
	if logFile == nil {
		return
	}

	entry := map[string]string{
		"time":  time.Now().UTC().Format(time.RFC3339),
		"error": msg,
	}
	json.NewEncoder(logFile).Encode(entry)
}

func openLogFile() {
	dir, err := logDir()
	if err != nil {
		return
	}
	logFile, _ = os.OpenFile(
		filepath.Join(dir, "errors.jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
}
