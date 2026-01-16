// Package importer provides utilities for importing markdown files into llmd.
package importer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/progress"
	"github.com/jpl-au/llmd/internal/service"
)

// Options configures an import operation.
type Options struct {
	Prefix string // Target path prefix
	Flat   bool   // Flatten directory structure
	Hidden bool   // Include hidden files/directories
	DryRun bool   // Show what would be imported without importing
	Author string // Author for imported documents
	Msg    string // Commit message for imported documents
}

// Result contains the outcome of an import operation.
type Result struct {
	Imported int      // Number of files imported
	Paths    []string // Paths that were/would be imported
}

// Run executes the import operation.
// Uses os.Root for safe path traversal within the source directory.
func Run(ctx context.Context, w io.Writer, svc service.Service, src string, opts Options) (Result, error) {
	var result Result

	info, err := os.Stat(src)
	if err != nil {
		return result, err
	}

	// Single file import
	if !info.IsDir() {
		return importSingleFile(ctx, w, svc, src, opts)
	}

	// Directory import using os.Root for safe traversal
	root, err := os.OpenRoot(src)
	if err != nil {
		return result, fmt.Errorf("opening source root: %w", err)
	}
	defer root.Close()

	files, err := scanRoot(root, "", opts.Hidden)
	if err != nil {
		return result, fmt.Errorf("scanning %s: %w", src, err)
	}

	if len(files) == 0 {
		return result, nil
	}

	prog := progress.New("Importing", len(files))
	defer prog.Done()

	for _, rel := range files {
		path := calcDocPath(rel, opts.Prefix, opts.Flat)
		result.Paths = append(result.Paths, path)

		if opts.DryRun {
			fmt.Fprintf(w, "Would import: %s -> %s\n", filepath.Join(src, rel), path)
			prog.Increment()
			prog.Print()
			continue
		}

		content, err := readFileInRoot(root, rel)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", rel, err)
		}

		if err := svc.Write(ctx, path, content, opts.Author, opts.Msg); err != nil {
			return result, fmt.Errorf("writing %s: %w", path, err)
		}

		prog.Increment()
		prog.Print()
		fmt.Fprintf(w, "Imported: %s -> %s\n", filepath.Join(src, rel), path)
		result.Imported++
	}

	return result, nil
}

// importSingleFile imports a single markdown file.
func importSingleFile(ctx context.Context, w io.Writer, svc service.Service, file string, opts Options) (Result, error) {
	var result Result

	if !strings.HasSuffix(strings.ToLower(file), ".md") {
		return result, nil
	}

	name := filepath.Base(file)
	path := calcDocPath(name, opts.Prefix, opts.Flat)
	result.Paths = append(result.Paths, path)

	if opts.DryRun {
		fmt.Fprintf(w, "Would import: %s -> %s\n", file, path)
		return result, nil
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return result, fmt.Errorf("reading %s: %w", file, err)
	}

	if err := svc.Write(ctx, path, string(content), opts.Author, opts.Msg); err != nil {
		return result, fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Fprintf(w, "Imported: %s -> %s\n", file, path)
	result.Imported = 1
	return result, nil
}

// scanRoot recursively finds all markdown files within an os.Root.
// Returns relative paths from the root.
func scanRoot(root *os.Root, dir string, includeHidden bool) ([]string, error) {
	var files []string

	path := dir
	if path == "" {
		path = "."
	}

	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files/dirs unless requested
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		rel := name
		if dir != "" {
			rel = filepath.Join(dir, name)
		}

		if entry.IsDir() {
			subfiles, err := scanRoot(root, rel, includeHidden)
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

// readFileInRoot reads a file's content within an os.Root.
func readFileInRoot(root *os.Root, name string) (string, error) {
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

// calcDocPath calculates the document path for importing a file.
func calcDocPath(relPath, prefix string, flat bool) string {
	// Remove .md extension
	path := strings.TrimSuffix(relPath, ".md")
	path = strings.TrimSuffix(path, ".MD")
	path = filepath.ToSlash(path)

	if flat {
		path = filepath.Base(path)
	}

	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/")
		path = prefix + "/" + path
	}

	return path
}
