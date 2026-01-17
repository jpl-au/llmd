// Package exporter provides utilities for exporting documents to the filesystem.
package exporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/progress"
	"github.com/jpl-au/llmd/internal/service"
)

// Options configures an export operation.
type Options struct {
	Version int  // Specific version to export (0 = latest)
	Force   bool // Overwrite existing files
}

// Result contains the outcome of an export operation.
type Result struct {
	Exported int      // Number of files exported
	Paths    []string // Filesystem paths that were written
}

// Run executes the export operation.
// If path ends with "/" it exports all documents with that prefix.
// Otherwise it exports a single document.
func Run(ctx context.Context, w io.Writer, svc service.Service, path, dst string, opts Options) (Result, error) {
	if strings.HasSuffix(path, "/") || path == "/" {
		return exportPrefix(ctx, w, svc, strings.TrimSuffix(path, "/"), dst, opts)
	}
	return exportSingle(ctx, w, svc, path, dst, opts)
}

// exportSingle exports a single document to the filesystem.
// Uses os.Root for safe path traversal, consistent with exportPrefix.
func exportSingle(ctx context.Context, w io.Writer, svc service.Service, docPath, dst string, opts Options) (Result, error) {
	var result Result

	// Resolve path or key to get actual document path
	doc, _, err := svc.Resolve(ctx, docPath, false)
	if err != nil {
		return result, fmt.Errorf("resolving document: %w", err)
	}
	docPath = doc.Path

	content, err := getContent(ctx, svc, docPath, opts.Version)
	if err != nil {
		return result, fmt.Errorf("getting document: %w", err)
	}

	outPath, dir, name, err := calcSingleOutputPath(dst, docPath)
	if err != nil {
		return result, fmt.Errorf("calculating output path: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return result, fmt.Errorf("creating directory: %w", err)
	}

	// Open directory as root for safe file operations
	root, err := os.OpenRoot(dir)
	if err != nil {
		return result, fmt.Errorf("opening destination: %w", err)
	}
	defer root.Close()

	if err := writeFileInRoot(root, name, content, opts.Force); err != nil {
		return result, err
	}

	result.Exported = 1
	result.Paths = []string{outPath}
	fmt.Fprintf(w, "Exported: %s -> %s\n", docPath, outPath)

	return result, nil
}

// exportPrefix exports all documents matching a prefix to the filesystem.
// Uses os.Root for safe path traversal within the destination directory.
func exportPrefix(ctx context.Context, w io.Writer, svc service.Service, pfx, dst string, opts Options) (Result, error) {
	var result Result

	docs, err := svc.List(ctx, pfx, false, false)
	if err != nil {
		return result, err
	}

	if len(docs) == 0 {
		return result, fmt.Errorf("no documents found with prefix: %s", pfx)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return result, fmt.Errorf("creating destination directory: %w", err)
	}

	// Open destination as root for safe file operations
	root, err := os.OpenRoot(dst)
	if err != nil {
		return result, fmt.Errorf("opening destination root: %w", err)
	}
	defer root.Close()

	prog := progress.New("Exporting", len(docs))
	defer prog.Done()

	for _, d := range docs {
		rel := calcRelativePath(d.Path, pfx)
		outName := rel + ".md"

		content, err := getContent(ctx, svc, d.Path, 0)
		if err != nil {
			return result, fmt.Errorf("getting %s: %w", d.Path, err)
		}

		if err := writeFileInRoot(root, outName, content, opts.Force); err != nil {
			return result, err
		}

		prog.Increment()
		prog.Print()
		outPath := filepath.Join(dst, outName)
		result.Paths = append(result.Paths, outPath)
		result.Exported++
		fmt.Fprintf(w, "Exported: %s -> %s\n", d.Path, outPath)
	}

	return result, nil
}

// getContent retrieves document content, optionally at a specific version.
func getContent(ctx context.Context, svc service.Service, path string, version int) (string, error) {
	if version > 0 {
		d, err := svc.Version(ctx, path, version)
		if err != nil {
			return "", err
		}
		return d.Content, nil
	}
	d, err := svc.Latest(ctx, path, false)
	if err != nil {
		return "", err
	}
	return d.Content, nil
}

// calcSingleOutputPath determines the output path for a single document export.
// Returns the full path, directory, and filename for use with os.Root.
func calcSingleOutputPath(dst, docPath string) (fullPath, dir, name string, err error) { //nolint:unparam
	info, statErr := os.Stat(dst)
	switch {
	case statErr == nil && info.IsDir():
		// Destination is a directory - add filename inside it
		name = filepath.Base(docPath) + ".md"
		return filepath.Join(dst, name), dst, name, nil
	case !strings.HasSuffix(dst, ".md"):
		// Non-existent path without .md - add extension
		fullPath = dst + ".md"
	default:
		fullPath = dst
	}
	dir = filepath.Dir(fullPath)
	name = filepath.Base(fullPath)
	return fullPath, dir, name, nil
}

// calcRelativePath strips the prefix from a document path.
func calcRelativePath(docPath, prefix string) string {
	if prefix == "" {
		return docPath
	}
	rel := strings.TrimPrefix(docPath, prefix+"/")
	if rel == docPath {
		rel = strings.TrimPrefix(docPath, prefix)
	}
	return rel
}

// writeFileInRoot writes content to a file within an os.Root, safely preventing
// path traversal attacks. Creates parent directories as needed.
func writeFileInRoot(root *os.Root, name, content string, force bool) error {
	// Check if file exists when not forcing
	if !force {
		if _, err := root.Stat(name); err == nil {
			return fmt.Errorf("file exists: %s (use --force to overwrite)", name)
		}
	}

	// Create parent directories within root
	dir := filepath.Dir(name)
	if dir != "." && dir != "" {
		if err := mkdirAllInRoot(root, dir); err != nil {
			return err
		}
	}

	// Write file using os.Root for path safety
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	f, err := root.OpenFile(name, flags, 0644)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", name, err)
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
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
