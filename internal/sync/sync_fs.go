// sync_fs.go provides filesystem operations for the document mirror.
//
// Separated from sync.go to isolate low-level file I/O. These functions
// handle writing, moving, and removing .md files in the filesystem mirror
// that shadows the database content.
//
// Security: All operations use os.Root (Go 1.24+) for path confinement.
// This prevents path traversal attacks - operations cannot escape the
// designated sync directory regardless of the document paths stored in
// the database. This is defence-in-depth alongside path validation.

package sync

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// readFileInRoot reads a document file's content within an os.Root.
func readFileInRoot(root *os.Root, path string) (string, error) {
	name := path + ".md"
	f, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	content := make([]byte, info.Size())
	_, err = io.ReadFull(f, content)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// WriteFile writes content to the filesystem mirror.
// Uses os.Root for safe path traversal.
func WriteFile(filesDir, path, content string) error {
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("creating files directory: %w", err)
	}

	root, err := os.OpenRoot(filesDir)
	if err != nil {
		return fmt.Errorf("opening files directory: %w", err)
	}
	defer root.Close()

	name := path + ".md"

	// Create parent directories
	dir := filepath.Dir(name)
	if dir != "." && dir != "" {
		if err := mkdirAllInRoot(root, dir); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	f, err := root.OpenFile(name, flags, 0644)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", name, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("writing file %s: %w", name, err)
	}
	return nil
}

// RemoveFile removes a file from the filesystem mirror.
// Returns nil if the file doesn't exist.
func RemoveFile(filesDir, path string) error {
	root, err := os.OpenRoot(filesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer root.Close()

	name := path + ".md"
	err = root.Remove(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// MoveFile moves a file in the filesystem mirror.
// Returns nil if the source file doesn't exist.
func MoveFile(filesDir, src, dst string) error {
	root, err := os.OpenRoot(filesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("opening files directory: %w", err)
	}
	defer root.Close()

	srcName := src + ".md"
	dstName := dst + ".md"

	// Read source file content
	content, err := readSourceFile(root, srcName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	// Create parent directories for destination
	dir := filepath.Dir(dstName)
	if dir != "." && dir != "" {
		if err := mkdirAllInRoot(root, dir); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Write destination file
	if err := writeDestFile(root, dstName, content); err != nil {
		return err
	}

	// Remove source file
	if err := root.Remove(srcName); err != nil {
		return fmt.Errorf("removing source file: %w", err)
	}
	return nil
}

// readSourceFile reads file content using defer for safe cleanup.
func readSourceFile(root *os.Root, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}

	content := make([]byte, info.Size())
	if _, err := io.ReadFull(f, content); err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return content, nil
}

// writeDestFile writes content to a file using defer for safe cleanup.
func writeDestFile(root *os.Root, name string, content []byte) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	f, err := root.OpenFile(name, flags, 0644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", name, err)
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// mkdirAllInRoot creates a directory and all parents within an os.Root.
func mkdirAllInRoot(root *os.Root, path string) error {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i := range parts {
		dir := filepath.Join(parts[:i+1]...)
		if err := root.Mkdir(dir, 0755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}
