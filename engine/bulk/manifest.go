// manifest.go records what the last mirror pull or push left in step
// between the store and the mirror directory, so a push can tell a
// file deleted from the mirror from a document that was never pulled.
//
// The manifest sits beside the mirror directory, not inside it
// (.llmd/llmd.manifest for .llmd/llmd/), so it is never committed with
// the mirrored files: it describes this checkout, not the repository.

package bulk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// manifest records the last point the mirror and the store matched.
type manifest struct {
	// Docs maps a store path to the content hash it had when the
	// mirror file and the document last matched.
	Docs map[string]string `json:"docs"`

	// Meta is the .meta.yaml as the last pull or push left it: the
	// base push compares the file with to see what changed in it.
	Meta meta `json:"meta,omitempty"`
}

// manifestPath returns the manifest file for a mirror directory.
func manifestPath(dir string) string {
	return filepath.Clean(dir) + ".manifest"
}

// readManifest loads the manifest for dir. A missing manifest is an
// empty one: nothing has been pulled, so a push deletes nothing.
func readManifest(dir string) (*manifest, error) {
	m := &manifest{Docs: map[string]string{}}
	data, err := os.ReadFile(manifestPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading mirror manifest: %w", err)
	}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("reading mirror manifest %s: %w", manifestPath(dir), err)
	}
	if m.Docs == nil {
		m.Docs = map[string]string{}
	}
	return m, nil
}

// write saves the manifest for dir.
func (m *manifest) write(dir string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath(dir), append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing mirror manifest: %w", err)
	}
	return nil
}

// underDir reports whether the store path lies under the directory
// prefix ("docs" or "docs/"); an empty prefix covers every path.
func underDir(path, prefix string) bool {
	prefix = strings.Trim(prefix, "/")
	return prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/")
}
