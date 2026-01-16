// Package repo provides repository initialisation and discovery for llmd.
//
// An llmd repository is a .llmd directory containing one or more SQLite databases.
// This package handles:
//   - Initialising new repositories (creating .llmd/ and the database)
//   - Discovering existing repositories by walking up the directory tree
//   - Managing multiple named databases (llmd.db, llmd-docs.db, etc.)
//   - Controlling git visibility via .gitignore (local vs shared databases)
//
// The discovery algorithm mirrors git's approach: starting from the current
// directory, walk up until a .llmd directory containing the target database
// is found, or the filesystem root is reached.
package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/store"
)

const (
	// Dir is the directory name for the llmd repository.
	Dir = ".llmd"
	// DBFile is the default database filename.
	DBFile = "llmd.db"
)

// DBFileName returns the database filename for a given name.
// Empty name returns the default "llmd.db".
// A name like "docs" returns "llmd-docs.db".
// A name already ending in ".db" is returned as-is.
func DBFileName(name string) string {
	if name == "" {
		return DBFile
	}
	if strings.HasSuffix(name, ".db") {
		return name
	}
	return "llmd-" + name + ".db"
}

// ErrNotInitialised is returned when no llmd repository is found.
var ErrNotInitialised = errors.New("llmd not initialised (run 'llmd init')")

// Init initialises a new llmd repository.
//
// Why init does not write config: Following the git model, init only creates
// the database. Config is a separate concern managed via "llmd config".
// This keeps responsibilities clear:
//   - init: create the database
//   - --local: mark database as gitignored
//   - config command: manage settings (global ~/.llmd/config.yaml or local .llmd/config.yaml)
//
// Parameters:
//   - force: reinitialise existing repository
//   - db: database name (empty for default "llmd.db")
//   - local: add database to .gitignore (not committed)
//   - dir: target directory (empty for current directory)
func Init(force bool, db string, local bool, dir string) error {
	if dir == "" {
		dir = "."
	}
	llmdDir := filepath.Join(dir, Dir)
	dbPath := filepath.Join(llmdDir, DBFileName(db))

	// Check if already exists
	if _, err := os.Stat(dbPath); err == nil {
		if !force {
			return fmt.Errorf("database %s already exists (use --force to reinitialise)", DBFileName(db))
		}
		// Remove existing DB for reinit
		if err := os.Remove(dbPath); err != nil {
			return fmt.Errorf("remove database: %w", err)
		}
	}

	// Create directory
	if err := os.MkdirAll(llmdDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create and initialise DB
	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if err := s.Init(); err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// Create .gitignore if it doesn't exist.
	// Only create on first init - subsequent inits (for additional databases)
	// should not overwrite and lose custom entries like local database markers.
	gitignore := filepath.Join(llmdDir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		s := `# llmd - ignore mirrored files and local config
# Database files (*.db) are the source of truth and should be committed
*.md
config.yaml
`
		if err := os.WriteFile(gitignore, []byte(s), 0644); err != nil {
			return fmt.Errorf("write gitignore: %w", err)
		}
	}

	// Mark database as local if requested (add to gitignore).
	//
	// Why --local only affects gitignore: The --local flag controls whether
	// the database file is committed to git. It does not create config.
	// Config is managed separately via "llmd config".
	if local {
		if err := IgnoreDB(db, llmdDir); err != nil {
			return fmt.Errorf("ignore database: %w", err)
		}
	}

	return nil
}

// Discover walks up the directory tree looking for a .llmd database.
// The db parameter specifies which database to find (empty for default).
// Returns the full path to the database if found.
func Discover(db string) (string, error) {
	dbFile := DBFileName(db)
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		dbPath := filepath.Join(dir, Dir, dbFile)
		if _, err := os.Stat(dbPath); err == nil {
			return dbPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialised
		}
		dir = parent
	}
}

// DiscoverDir finds the .llmd directory, walking up the tree.
// Returns the full path to the .llmd directory.
func DiscoverDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		llmdDir := filepath.Join(dir, Dir)
		if info, err := os.Stat(llmdDir); err == nil && info.IsDir() {
			return llmdDir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialised
		}
		dir = parent
	}
}

// DBInfo holds database metadata.
type DBInfo struct {
	Name  string // Short name (empty for default, "docs" for llmd-docs.db)
	File  string // Filename (llmd.db, llmd-docs.db)
	Path  string // Full path
	Local bool   // True if gitignored
}

// ListDBs returns all databases in the .llmd directory with their status.
// If dir is empty, discovers .llmd directory from current working directory.
func ListDBs(dir string) ([]DBInfo, error) {
	if dir == "" {
		var err error
		dir, err = DiscoverDir()
		if err != nil {
			return nil, fmt.Errorf("discover .llmd directory: %w", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read .llmd directory: %w", err)
	}

	var dbs []DBInfo
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".db") {
			continue
		}

		// Extract short name from filename
		name := ""
		if e.Name() == DBFile {
			name = ""
		} else if strings.HasPrefix(e.Name(), "llmd-") {
			name = strings.TrimSuffix(strings.TrimPrefix(e.Name(), "llmd-"), ".db")
		} else {
			continue // Not an llmd database
		}

		ignored, err := IsIgnored(name, dir)
		if err != nil {
			// If we can't determine ignored status, default to false (shared).
			// This can happen if .gitignore is malformed or unreadable.
			ignored = false
		}
		dbs = append(dbs, DBInfo{
			Name:  name,
			File:  e.Name(),
			Path:  filepath.Join(dir, e.Name()),
			Local: ignored,
		})
	}

	return dbs, nil
}
