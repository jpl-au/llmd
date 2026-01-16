package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/document"
)

// tempStore creates a temporary llmd store for examples.
func tempStore() (*document.Service, func()) {
	dir, err := os.MkdirTemp("", "llmd-example-*")
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	if err := document.Init(false, "", false, ""); err != nil {
		panic(err)
	}
	svc, err := document.New("")
	if err != nil {
		panic(err)
	}
	cleanup := func() {
		svc.Close()
		os.RemoveAll(dir)
	}
	return svc, cleanup
}

func Example_basicUsage() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Write a document
	err := svc.Write(ctx, "docs/hello", "Hello, World!", "alice", "Initial commit")
	if err != nil {
		panic(err)
	}

	// Read it back
	doc, err := svc.Latest(ctx, "docs/hello", false)
	if err != nil {
		panic(err)
	}
	fmt.Println(doc.Content)
	fmt.Println(doc.Version)
	// Output:
	// Hello, World!
	// 1
}

func Example_exists() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Check before creating
	exists, _ := svc.Exists(ctx, "docs/new")
	fmt.Println("Before:", exists)

	// Create document
	_ = svc.Write(ctx, "docs/new", "content", "alice", "")

	// Check after creating
	exists, _ = svc.Exists(ctx, "docs/new")
	fmt.Println("After:", exists)
	// Output:
	// Before: false
	// After: true
}

func Example_copy() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Create original
	_ = svc.Write(ctx, "docs/original", "Important content", "alice", "")

	// Copy to new location (bob performs the copy)
	err := svc.Copy(ctx, "docs/original", "docs/backup", "bob")
	if err != nil {
		panic(err)
	}

	// Verify copy
	doc, _ := svc.Latest(ctx, "docs/backup", false)
	fmt.Println(doc.Content)
	fmt.Println(doc.Author) // Shows who copied, not original author
	fmt.Println(doc.Message)
	// Output:
	// Important content
	// bob
	// Copied from docs/original
}

func Example_count() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Create some documents
	_ = svc.Write(ctx, "docs/a", "A", "alice", "")
	_ = svc.Write(ctx, "docs/b", "B", "alice", "")
	_ = svc.Write(ctx, "notes/x", "X", "alice", "")

	// Count all
	all, _ := svc.Count(ctx, "")
	fmt.Println("All:", all)

	// Count by prefix
	docs, _ := svc.Count(ctx, "docs/")
	fmt.Println("Docs:", docs)
	// Output:
	// All: 3
	// Docs: 2
}

func Example_meta() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Create a document with some content
	_ = svc.Write(ctx, "docs/large", "This is some content that we might not need to fetch", "alice", "Added content")

	// Get metadata without fetching content
	meta, err := svc.Meta(ctx, "docs/large")
	if err != nil {
		panic(err)
	}
	fmt.Println("Path:", meta.Path)
	fmt.Println("Version:", meta.Version)
	fmt.Println("Author:", meta.Author)
	fmt.Printf("Size: %d bytes\n", meta.Size)
	// Output:
	// Path: docs/large
	// Version: 1
	// Author: alice
	// Size: 52 bytes
}

func Example_transaction() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Use transaction for atomic operations on custom tables
	err := svc.Tx(ctx, func(tx *sql.Tx) error {
		// This runs in a transaction - all or nothing
		// Real usage would be for extension tables, e.g.:
		// _, err := tx.Exec("INSERT INTO tasks (title) VALUES (?)", "Task 1")
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Transaction completed")
	// Output:
	// Transaction completed
}

func Example_search() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Create documents
	_ = svc.Write(ctx, "docs/go", "Go is a statically typed language", "alice", "")
	_ = svc.Write(ctx, "docs/rust", "Rust is a systems programming language", "alice", "")
	_ = svc.Write(ctx, "docs/python", "Python is dynamically typed", "alice", "")

	// Search for "typed"
	results, _ := svc.Search(ctx, "typed", "", false, false)
	for _, doc := range results {
		fmt.Println(filepath.Base(doc.Path))
	}
	// Output:
	// go
	// python
}

func Example_history() {
	svc, cleanup := tempStore()
	defer cleanup()
	ctx := context.Background()

	// Create multiple versions
	_ = svc.Write(ctx, "docs/evolving", "Version 1", "alice", "Initial")
	_ = svc.Write(ctx, "docs/evolving", "Version 2", "bob", "Update")
	_ = svc.Write(ctx, "docs/evolving", "Version 3", "alice", "Final")

	// Get history (newest first)
	history, _ := svc.History(ctx, "docs/evolving", 0, false)
	for _, doc := range history {
		fmt.Printf("v%d by %s: %s\n", doc.Version, doc.Author, doc.Message)
	}
	// Output:
	// v3 by alice: Final
	// v2 by bob: Update
	// v1 by alice: Initial
}
