// Package sync provides filesystem synchronisation utilities for llmd.
// It handles both mirroring documents to the filesystem and detecting
// changes made directly to filesystem files.
//
// Security: All filesystem operations use os.OpenRoot to prevent path traversal
// attacks. Document paths like "../../etc/passwd" could otherwise escape the
// files directory. os.OpenRoot creates a "chroot-like" handle that rejects any
// path resolving outside the root, including symlinks pointing outside.
// This is critical because document paths come from user input and the database.
package sync

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/internal/progress"
	"github.com/jpl-au/llmd/internal/service"
)

// Options configures a sync operation.
type Options struct {
	DryRun bool   // Show what would be synced without syncing
	Author string // Author for synced documents
	Msg    string // Commit message for synced documents
}

// Result contains the outcome of a sync operation.
type Result struct {
	Updated int // Number of documents updated
	Added   int // Number of documents added
}

// Changes represents detected filesystem changes.
type Changes struct {
	Changed []string // Paths of documents that were modified
	Added   []string // Paths of new documents
}

// Empty returns true if there are no changes.
func (c Changes) Empty() bool {
	return len(c.Changed) == 0 && len(c.Added) == 0
}

// Total returns the total number of changes.
func (c Changes) Total() int {
	return len(c.Changed) + len(c.Added)
}

// Run executes the sync operation, importing filesystem changes into the database.
// Uses os.Root for safe path traversal within the files directory.
func Run(ctx context.Context, w io.Writer, svc service.Service, filesDir string, db map[string]string, opts Options) (Result, error) {
	var result Result

	root, err := os.OpenRoot(filesDir)
	if err != nil {
		return result, fmt.Errorf("opening files directory: %w", err)
	}
	defer root.Close()

	changes, err := detectChangesInRoot(root, db)
	if err != nil {
		return result, err
	}

	if changes.Empty() {
		return result, nil
	}

	msg := opts.Msg
	if msg == "" {
		msg = "Synced from filesystem"
	}

	prog := progress.New("Syncing", changes.Total())
	defer prog.Done()

	for _, p := range changes.Changed {
		content, err := readFileInRoot(root, p)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", p, err)
		}

		if opts.DryRun {
			fmt.Fprintf(w, "Would update: %s\n", p)
		} else {
			if err := svc.Write(ctx, p, content, opts.Author, msg); err != nil {
				return result, fmt.Errorf("updating %s: %w", p, err)
			}
			fmt.Fprintf(w, "Updated: %s\n", p)
			result.Updated++
		}
		prog.Increment()
		prog.Print()
	}

	for _, p := range changes.Added {
		content, err := readFileInRoot(root, p)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", p, err)
		}

		if opts.DryRun {
			fmt.Fprintf(w, "Would add: %s\n", p)
		} else {
			if err := svc.Write(ctx, p, content, opts.Author, msg); err != nil {
				return result, fmt.Errorf("adding %s: %w", p, err)
			}
			fmt.Fprintf(w, "Added: %s\n", p)
			result.Added++
		}
		prog.Increment()
		prog.Print()
	}

	return result, nil
}
