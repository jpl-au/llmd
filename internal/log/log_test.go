package log

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	// Use temp directory for test database
	tmpDir := t.TempDir()
	origDBPath := dbPathFunc
	dbPathFunc = func() string {
		return filepath.Join(tmpDir, "log", "test.db")
	}
	defer func() { dbPathFunc = origDBPath }()

	t.Run("open and close", func(t *testing.T) {
		err := Open()
		require.NoError(t, err)
		defer Close()

		// Verify database file exists
		assert.FileExists(t, DBPath())
	})

	t.Run("log entry", func(t *testing.T) {
		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		Log(Entry{
			Source:  "document:cat",
			Author:  "test-user",
			Action:  "read",
			Path:    "docs/readme",
			Version: 3,
			Success: true,
		})

		// Verify entry was written
		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM log").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		var source, action, path string
		var version int
		var success int
		err = db.QueryRow("SELECT source, action, path, version, success FROM log WHERE id = 1").
			Scan(&source, &action, &path, &version, &success)
		require.NoError(t, err)
		assert.Equal(t, "document:cat", source)
		assert.Equal(t, "read", action)
		assert.Equal(t, "docs/readme", path)
		assert.Equal(t, 3, version)
		assert.Equal(t, 1, success)
	})

	t.Run("log error entry", func(t *testing.T) {
		// Reset global for clean test
		Close()

		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		Log(Entry{
			Source:  "document:cat",
			Action:  "read",
			Path:    "docs/missing",
			Success: false,
			Error:   "document not found",
		})

		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var success int
		var errMsg string
		err = db.QueryRow("SELECT success, error FROM log ORDER BY id DESC LIMIT 1").
			Scan(&success, &errMsg)
		require.NoError(t, err)
		assert.Equal(t, 0, success)
		assert.Equal(t, "document not found", errMsg)
	})

	t.Run("log with detail", func(t *testing.T) {
		Close()

		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		Log(Entry{
			Source:  "search:grep",
			Action:  "search",
			Success: true,
			Detail:  map[string]any{"pattern": "TODO", "count": 42},
		})

		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var detail string
		err = db.QueryRow("SELECT detail FROM log ORDER BY id DESC LIMIT 1").Scan(&detail)
		require.NoError(t, err)
		assert.Contains(t, detail, "TODO")
		assert.Contains(t, detail, "42")
	})

	t.Run("log without logger is noop", func(t *testing.T) {
		Close()

		// Should not panic
		Log(Entry{
			Source:  "test:cmd",
			Action:  "test",
			Success: true,
		})
	})

	t.Run("open is idempotent", func(t *testing.T) {
		err := Open()
		require.NoError(t, err)

		err = Open() // second call should succeed
		require.NoError(t, err)

		Close()
	})
}

func TestHash(t *testing.T) {
	h1 := hash("/home/user/project/.llmd")
	h2 := hash("/home/user/project/.llmd")
	h3 := hash("/home/user/other/.llmd")

	assert.Equal(t, h1, h2, "same input should produce same hash")
	assert.NotEqual(t, h1, h3, "different input should produce different hash")
	assert.Len(t, h1, 16, "BLAKE2b-64 should produce 16 hex chars")
}

func TestDBPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(home, ".llmd", "log", "llmd-log.db")

	// Use default path function
	origDBPath := dbPathFunc
	dbPathFunc = defaultDBPath
	defer func() { dbPathFunc = origDBPath }()

	assert.Equal(t, expected, DBPath())
}

func TestBuilder(t *testing.T) {
	// Use temp directory for test database
	tmpDir := t.TempDir()
	origDBPath := dbPathFunc
	dbPathFunc = func() string {
		return filepath.Join(tmpDir, "log", "test.db")
	}
	defer func() { dbPathFunc = origDBPath }()

	t.Run("fluent API success", func(t *testing.T) {
		Close()
		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		Event("document:cat", "read").
			Author("test-user").
			Path("docs/readme").
			Version(5).
			Write(nil) // success

		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var source, author, action, path string
		var version, success int
		err = db.QueryRow("SELECT source, author, action, path, version, success FROM log ORDER BY id DESC LIMIT 1").
			Scan(&source, &author, &action, &path, &version, &success)
		require.NoError(t, err)
		assert.Equal(t, "document:cat", source)
		assert.Equal(t, "test-user", author)
		assert.Equal(t, "read", action)
		assert.Equal(t, "docs/readme", path)
		assert.Equal(t, 5, version)
		assert.Equal(t, 1, success)
	})

	t.Run("fluent API with error", func(t *testing.T) {
		Close()
		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		testErr := sql.ErrNoRows // use any error
		Event("document:cat", "read").
			Author("test-user").
			Path("docs/missing").
			Write(testErr)

		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var success int
		var errMsg string
		err = db.QueryRow("SELECT success, error FROM log ORDER BY id DESC LIMIT 1").
			Scan(&success, &errMsg)
		require.NoError(t, err)
		assert.Equal(t, 0, success)
		assert.Equal(t, testErr.Error(), errMsg)
	})

	t.Run("fluent API with Detail", func(t *testing.T) {
		Close()
		err := Open()
		require.NoError(t, err)
		defer Close()

		SetProject("/test/project/.llmd")

		Event("search:find", "search").
			Author("test-user").
			Detail("query", "TODO").
			Detail("count", 42).
			Write(nil)

		db, err := sql.Open("sqlite", DBPath())
		require.NoError(t, err)
		defer db.Close()

		var detail string
		err = db.QueryRow("SELECT detail FROM log ORDER BY id DESC LIMIT 1").Scan(&detail)
		require.NoError(t, err)
		assert.Contains(t, detail, "TODO")
		assert.Contains(t, detail, "42")
	})
}
