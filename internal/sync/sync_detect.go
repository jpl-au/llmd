// sync_detect.go implements filesystem change detection for sync operations.
//
// Separated from sync.go to isolate the directory scanning and diff logic.
// Change detection compares the filesystem state against the database to find
// added, changed, and (implicitly) deleted files.
//
// Security: Uses os.Root (Go 1.24+) to prevent path traversal attacks. All
// filesystem access is confined to the sync directory - even maliciously
// crafted symlinks or ".." paths cannot escape. MaxScanDepth prevents DoS
// from deeply nested directory structures.

package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/path"
)

// MaxScanDepth limits directory recursion to prevent DoS on deep trees.
const MaxScanDepth = 100

// detectChangesInRoot scans the os.Root for changes compared to the database.
func detectChangesInRoot(root *os.Root, db map[string]string) (Changes, error) {
	var changes Changes

	files, err := scanRootDir(root, "", 0)
	if err != nil {
		return changes, err
	}

	for _, rel := range files {
		docPath := strings.TrimSuffix(rel, ".md")
		docPath = strings.TrimSuffix(docPath, ".MD")
		docPath = filepath.ToSlash(docPath)

		// Normalise path to prevent malformed paths from filesystem reaching the database.
		// Defence-in-depth: os.OpenRoot prevents escape, but we also validate path format.
		docPath, err := path.Normalise(docPath)
		if err != nil {
			// Skip files with invalid paths (e.g., containing "..")
			continue
		}

		content, err := readFileInRoot(root, docPath)
		if err != nil {
			return changes, err
		}

		if stored, exists := db[docPath]; exists {
			if content != stored {
				changes.Changed = append(changes.Changed, docPath)
			}
		} else {
			changes.Added = append(changes.Added, docPath)
		}
	}

	return changes, nil
}

// scanRootDir recursively finds all markdown files within an os.Root.
// Depth is limited by MaxScanDepth to prevent DoS on deeply nested trees.
func scanRootDir(root *os.Root, dir string, depth int) ([]string, error) {
	if depth > MaxScanDepth {
		return nil, fmt.Errorf("directory depth exceeds limit of %d", MaxScanDepth)
	}

	var files []string

	path := dir
	if path == "" {
		path = "."
	}

	f, err := root.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files/dirs
		if strings.HasPrefix(name, ".") {
			continue
		}

		rel := name
		if dir != "" {
			rel = filepath.Join(dir, name)
		}

		if entry.IsDir() {
			subfiles, err := scanRootDir(root, rel, depth+1)
			if err != nil {
				return nil, err
			}
			files = append(files, subfiles...)
		} else if strings.HasSuffix(strings.ToLower(name), ".md") {
			files = append(files, rel)
		}
	}

	return files, nil
}
