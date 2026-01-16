package store_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStore creates a temporary SQLite store for testing.
// Returns the store and a cleanup function.
func setupStore(t *testing.T) (*store.SQLiteStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "llmd-store-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	require.NoError(t, err)

	require.NoError(t, s.Init())

	cleanup := func() {
		s.Close()
		os.RemoveAll(tmpDir)
	}

	return s, cleanup
}

// writeOpts returns WriteOptions with test defaults.
func writeOpts(author, msg string) store.WriteOptions {
	return store.WriteOptions{Author: author, Message: msg}
}

// --- Basic CRUD Tests ---

func TestStore_WriteAndLatest(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/readme"
	content := "# README\nHello World"
	author := "alice"
	msg := "initial commit"

	// Write document
	err := s.Write(ctx, path, content, writeOpts(author, msg))
	require.NoError(t, err)

	// Read back
	doc, err := s.Latest(ctx, path, false)
	require.NoError(t, err)

	assert.Equal(t, path, doc.Path)
	assert.Equal(t, content, doc.Content)
	assert.Equal(t, author, doc.Author)
	assert.Equal(t, msg, doc.Message)
	assert.Equal(t, 1, doc.Version)
	assert.NotEmpty(t, doc.Key)
	assert.Nil(t, doc.DeletedAt)
}

func TestStore_VersionIncrement(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/evolving"

	// Write multiple versions
	require.NoError(t, s.Write(ctx, path, "v1 content", writeOpts("alice", "v1")))
	require.NoError(t, s.Write(ctx, path, "v2 content", writeOpts("bob", "v2")))
	require.NoError(t, s.Write(ctx, path, "v3 content", writeOpts("alice", "v3")))

	// Latest should be v3
	doc, err := s.Latest(ctx, path, false)
	require.NoError(t, err)
	assert.Equal(t, 3, doc.Version)
	assert.Equal(t, "v3 content", doc.Content)

	// Can retrieve specific versions
	v1, err := s.Version(ctx, path, 1)
	require.NoError(t, err)
	assert.Equal(t, "v1 content", v1.Content)
	assert.Equal(t, "alice", v1.Author)

	v2, err := s.Version(ctx, path, 2)
	require.NoError(t, err)
	assert.Equal(t, "v2 content", v2.Content)
	assert.Equal(t, "bob", v2.Author)
}

func TestStore_ByKey(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/test", "content", writeOpts("alice", "")))

	// Get the key from Latest
	doc, err := s.Latest(ctx, "docs/test", false)
	require.NoError(t, err)
	key := doc.Key

	// Retrieve by key
	byKey, err := s.ByKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, doc.Path, byKey.Path)
	assert.Equal(t, doc.Content, byKey.Content)
}

func TestStore_NotFound(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Latest on non-existent path
	_, err := s.Latest(ctx, "nonexistent", false)
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Version on non-existent path
	_, err = s.Version(ctx, "nonexistent", 1)
	assert.ErrorIs(t, err, store.ErrNotFound)

	// ByKey on non-existent key
	_, err = s.ByKey(ctx, "badkey00")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// --- List and History Tests ---

func TestStore_List(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create documents in different paths
	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "notes/x", "X", writeOpts("alice", "")))

	// List all
	all, err := s.List(ctx, "", false, false)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List by prefix
	docs, err := s.List(ctx, "docs/", false, false)
	require.NoError(t, err)
	assert.Len(t, docs, 2)

	notes, err := s.List(ctx, "notes/", false, false)
	require.NoError(t, err)
	assert.Len(t, notes, 1)
}

func TestStore_ListPaths(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))
	// Write multiple versions - should still only list path once
	require.NoError(t, s.Write(ctx, "docs/a", "A updated", writeOpts("bob", "")))

	paths, err := s.ListPaths(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/a", "docs/b"}, paths)
}

func TestStore_History(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/versioned"
	require.NoError(t, s.Write(ctx, path, "v1", writeOpts("alice", "first")))
	require.NoError(t, s.Write(ctx, path, "v2", writeOpts("bob", "second")))
	require.NoError(t, s.Write(ctx, path, "v3", writeOpts("alice", "third")))

	// Full history (newest first)
	history, err := s.History(ctx, path, 0, false)
	require.NoError(t, err)
	require.Len(t, history, 3)
	assert.Equal(t, 3, history[0].Version)
	assert.Equal(t, 2, history[1].Version)
	assert.Equal(t, 1, history[2].Version)

	// Limited history
	limited, err := s.History(ctx, path, 2, false)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestStore_Count(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "notes/x", "X", writeOpts("alice", "")))

	all, err := s.Count(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), all)

	docs, err := s.Count(ctx, "docs/")
	require.NoError(t, err)
	assert.Equal(t, int64(2), docs)
}

func TestStore_Exists(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	exists, err := s.Exists(ctx, "docs/test")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, s.Write(ctx, "docs/test", "content", writeOpts("alice", "")))

	exists, err = s.Exists(ctx, "docs/test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestStore_Meta(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	content := "Hello, World!"
	require.NoError(t, s.Write(ctx, "docs/test", content, writeOpts("alice", "create")))

	meta, err := s.Meta(ctx, "docs/test")
	require.NoError(t, err)

	assert.Equal(t, "docs/test", meta.Path)
	assert.Equal(t, 1, meta.Version)
	assert.Equal(t, "alice", meta.Author)
	assert.Equal(t, int64(len(content)), meta.Size)
}

// --- Soft-Delete Lifecycle Tests ---

func TestStore_DeleteRestore(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/deleteme"
	require.NoError(t, s.Write(ctx, path, "content", writeOpts("alice", "")))

	// Verify exists
	exists, _ := s.Exists(ctx, path)
	assert.True(t, exists)

	// Delete
	require.NoError(t, s.Delete(ctx, path, store.DeleteOptions{}))

	// Should not exist (without includeDeleted)
	exists, _ = s.Exists(ctx, path)
	assert.False(t, exists)

	// Should be visible with includeDeleted
	doc, err := s.Latest(ctx, path, true)
	require.NoError(t, err)
	assert.NotNil(t, doc.DeletedAt)

	// Restore
	require.NoError(t, s.Restore(ctx, path, store.RestoreOptions{}))

	// Should exist again
	exists, _ = s.Exists(ctx, path)
	assert.True(t, exists)

	doc, err = s.Latest(ctx, path, false)
	require.NoError(t, err)
	assert.Nil(t, doc.DeletedAt)
}

func TestStore_DeleteNotFound(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent", store.DeleteOptions{})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestStore_RestoreNotFound(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Restore non-existent
	err := s.Restore(ctx, "nonexistent", store.RestoreOptions{})
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Restore non-deleted
	require.NoError(t, s.Write(ctx, "docs/active", "content", writeOpts("alice", "")))
	err = s.Restore(ctx, "docs/active", store.RestoreOptions{})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestStore_DeleteVersion(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/multiversion"
	require.NoError(t, s.Write(ctx, path, "v1", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, path, "v2", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, path, "v3", writeOpts("alice", "")))

	// Delete only v2
	require.NoError(t, s.DeleteVersion(ctx, path, 2, store.DeleteVersionOptions{}))

	// v1 and v3 should still be accessible
	v1, err := s.Version(ctx, path, 1)
	require.NoError(t, err)
	assert.Nil(t, v1.DeletedAt)

	v3, err := s.Version(ctx, path, 3)
	require.NoError(t, err)
	assert.Nil(t, v3.DeletedAt)

	// v2 should be deleted (but retrievable via Version)
	v2, err := s.Version(ctx, path, 2)
	require.NoError(t, err)
	assert.NotNil(t, v2.DeletedAt)

	// Latest should skip deleted v2 and return v3
	latest, err := s.Latest(ctx, path, false)
	require.NoError(t, err)
	assert.Equal(t, 3, latest.Version)
}

func TestStore_ListDeletedOnly(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/active", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/deleted", "content", writeOpts("alice", "")))
	require.NoError(t, s.Delete(ctx, "docs/deleted", store.DeleteOptions{}))

	// deletedOnly should only show deleted
	deleted, err := s.List(ctx, "", false, true)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, "docs/deleted", deleted[0].Path)

	// includeDeleted should show both
	all, err := s.List(ctx, "", true, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

// --- Move and Copy Tests ---

func TestStore_Move(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/old", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/old", "updated", writeOpts("bob", "")))

	require.NoError(t, s.Move(ctx, "docs/old", "docs/new", store.MoveOptions{}))

	// Old path should not exist
	exists, _ := s.Exists(ctx, "docs/old")
	assert.False(t, exists)

	// New path should exist with all versions
	doc, err := s.Latest(ctx, "docs/new", false)
	require.NoError(t, err)
	assert.Equal(t, "docs/new", doc.Path)
	assert.Equal(t, 2, doc.Version)

	history, err := s.History(ctx, "docs/new", 0, false)
	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestStore_MoveNotFound(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	err := s.Move(ctx, "nonexistent", "docs/new", store.MoveOptions{})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestStore_MoveAlreadyExists(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))

	err := s.Move(ctx, "docs/a", "docs/b", store.MoveOptions{})
	assert.ErrorIs(t, err, store.ErrAlreadyExists)
}

func TestStore_Copy(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/original", "content", writeOpts("alice", "")))

	require.NoError(t, s.Copy(ctx, "docs/original", "docs/copy", "bob", store.CopyOptions{}))

	// Original should still exist
	orig, err := s.Latest(ctx, "docs/original", false)
	require.NoError(t, err)
	assert.Equal(t, "alice", orig.Author)

	// Copy should exist with new author
	copy, err := s.Latest(ctx, "docs/copy", false)
	require.NoError(t, err)
	assert.Equal(t, "content", copy.Content)
	assert.Equal(t, "bob", copy.Author)
	assert.Equal(t, 1, copy.Version)
}

func TestStore_CopyNotFound(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	err := s.Copy(ctx, "nonexistent", "docs/copy", "bob", store.CopyOptions{})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestStore_CopyAlreadyExists(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))

	err := s.Copy(ctx, "docs/a", "docs/b", "bob", store.CopyOptions{})
	assert.ErrorIs(t, err, store.ErrAlreadyExists)
}

// --- Tag Tests ---

func TestStore_Tags(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/tagged"
	require.NoError(t, s.Write(ctx, path, "content", writeOpts("alice", "")))

	opts := store.NewTagOptions()

	// Add tags
	require.NoError(t, s.Tag(ctx, path, "important", opts))
	require.NoError(t, s.Tag(ctx, path, "v1", opts))

	// List tags for document
	tags, err := s.ListTags(ctx, path, opts)
	require.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "important")
	assert.Contains(t, tags, "v1")

	// Find documents by tag
	paths, err := s.PathsWithTag(ctx, "important", opts)
	require.NoError(t, err)
	assert.Equal(t, []string{path}, paths)

	// Remove tag
	require.NoError(t, s.Untag(ctx, path, "v1", opts))

	tags, err = s.ListTags(ctx, path, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"important"}, tags)
}

func TestStore_TagIdempotent(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/test"
	require.NoError(t, s.Write(ctx, path, "content", writeOpts("alice", "")))

	opts := store.NewTagOptions()

	// Tag twice - should not error
	require.NoError(t, s.Tag(ctx, path, "mytag", opts))
	require.NoError(t, s.Tag(ctx, path, "mytag", opts))

	tags, err := s.ListTags(ctx, path, opts)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
}

func TestStore_ListAllTags(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))

	opts := store.NewTagOptions()
	require.NoError(t, s.Tag(ctx, "docs/a", "tag1", opts))
	require.NoError(t, s.Tag(ctx, "docs/b", "tag2", opts))
	require.NoError(t, s.Tag(ctx, "docs/b", "tag1", opts))

	// List all tags (empty path)
	allTags, err := s.ListTags(ctx, "", opts)
	require.NoError(t, err)
	assert.Len(t, allTags, 2)
}

// --- Link Tests ---

func TestStore_Links(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))

	opts := store.NewLinkOptions()

	// Create link
	id, err := s.Link(ctx, "docs/a", "docs/b", "related", opts)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// List links for document
	links, err := s.ListLinks(ctx, "docs/a", "", opts)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "docs/a", links[0].FromPath)
	assert.Equal(t, "docs/b", links[0].ToPath)
	assert.Equal(t, "related", links[0].Tag)

	// List links by tag
	byTag, err := s.ListLinksByTag(ctx, "related", opts)
	require.NoError(t, err)
	assert.Len(t, byTag, 1)

	// Unlink by ID
	require.NoError(t, s.UnlinkByID(ctx, id))

	links, err = s.ListLinks(ctx, "docs/a", "", opts)
	require.NoError(t, err)
	assert.Len(t, links, 0)
}

func TestStore_UnlinkByTag(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/a", "A", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/b", "B", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/c", "C", writeOpts("alice", "")))

	opts := store.NewLinkOptions()

	// Create multiple links with same tag
	_, err := s.Link(ctx, "docs/a", "docs/b", "depends-on", opts)
	require.NoError(t, err)
	_, err = s.Link(ctx, "docs/a", "docs/c", "depends-on", opts)
	require.NoError(t, err)

	// Unlink all with tag
	count, err := s.UnlinkByTag(ctx, "depends-on", opts)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	links, err := s.ListLinksByTag(ctx, "depends-on", opts)
	require.NoError(t, err)
	assert.Len(t, links, 0)
}

func TestStore_ListOrphanLinkPaths(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/linked", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/also-linked", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/orphan", "content", writeOpts("alice", "")))

	opts := store.NewLinkOptions()

	// Create link between two docs
	_, err := s.Link(ctx, "docs/linked", "docs/also-linked", "", opts)
	require.NoError(t, err)

	// Find orphans
	orphans, err := s.ListOrphanLinkPaths(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/orphan"}, orphans)
}

// --- Search Tests ---

func TestStore_Search(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/go", "Go is a statically typed language", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/rust", "Rust is a systems programming language", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/python", "Python is dynamically typed", writeOpts("alice", "")))

	// Search for "typed"
	results, err := s.Search(ctx, "typed", "", false, false)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Search with prefix - only Go and Rust contain "language"
	results, err = s.Search(ctx, "language", "docs/", false, false)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// --- Vacuum Tests ---

func TestStore_Vacuum(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create and delete documents
	require.NoError(t, s.Write(ctx, "docs/keep", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/delete1", "content", writeOpts("alice", "")))
	require.NoError(t, s.Write(ctx, "docs/delete2", "content", writeOpts("alice", "")))

	require.NoError(t, s.Delete(ctx, "docs/delete1", store.DeleteOptions{}))
	require.NoError(t, s.Delete(ctx, "docs/delete2", store.DeleteOptions{}))

	// Vacuum with no time restriction
	count, err := s.Vacuum(ctx, nil, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Deleted docs should be gone
	_, err = s.Latest(ctx, "docs/delete1", true)
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Active doc should remain
	_, err = s.Latest(ctx, "docs/keep", false)
	require.NoError(t, err)
}

func TestStore_VacuumOlderThan(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, s.Write(ctx, "docs/test", "content", writeOpts("alice", "")))
	require.NoError(t, s.Delete(ctx, "docs/test", store.DeleteOptions{}))

	// Vacuum with future time restriction - should not delete
	oneHour := time.Hour
	count, err := s.Vacuum(ctx, &oneHour, "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Doc should still exist (deleted but not vacuumed)
	doc, err := s.Latest(ctx, "docs/test", true)
	require.NoError(t, err)
	assert.NotNil(t, doc.DeletedAt)
}

// --- Edge Cases ---

func TestStore_EmptyContent(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Empty content is valid
	require.NoError(t, s.Write(ctx, "docs/empty", "", writeOpts("alice", "")))

	doc, err := s.Latest(ctx, "docs/empty", false)
	require.NoError(t, err)
	assert.Equal(t, "", doc.Content)
}

func TestStore_SpecialCharactersInPath(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Paths with various characters
	paths := []string{
		"docs/with spaces",
		"docs/with-dashes",
		"docs/with_underscores",
		"docs/CamelCase",
		"docs/123numeric",
	}

	for _, p := range paths {
		require.NoError(t, s.Write(ctx, p, "content", writeOpts("alice", "")))

		doc, err := s.Latest(ctx, p, false)
		require.NoError(t, err)
		assert.Equal(t, p, doc.Path)
	}
}

func TestStore_UniqueKeys(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create multiple documents and verify unique keys
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		path := "docs/doc" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		require.NoError(t, s.Write(ctx, path, "content", writeOpts("alice", "")))

		doc, err := s.Latest(ctx, path, false)
		require.NoError(t, err)

		assert.False(t, keys[doc.Key], "duplicate key: %s", doc.Key)
		keys[doc.Key] = true
	}
}

func TestStore_Transaction(t *testing.T) {
	s, cleanup := setupStore(t)
	defer cleanup()
	ctx := context.Background()

	// Test that Tx properly handles rollback on error
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO documents (key, path, content, version, author, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			"testkey1", "docs/tx-test", "content", 1, "alice", time.Now().Unix())
		if err != nil {
			return err
		}
		// Return error to trigger rollback
		return assert.AnError
	})
	assert.Error(t, err)

	// Document should not exist due to rollback
	exists, _ := s.Exists(ctx, "docs/tx-test")
	assert.False(t, exists)
}
