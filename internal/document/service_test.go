package document_test

import (
	"context"
	"os"
	"testing"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupService creates a temporary service and returns it along with a cleanup function.
func setupService(t *testing.T) (service.Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "llmd-test-*")
	require.NoError(t, err, "creating temp dir")

	cwd, err := os.Getwd()
	require.NoError(t, err, "getting cwd")

	require.NoError(t, os.Chdir(tmpDir), "chdir to temp")

	require.NoError(t, document.Init(true, "", false, ""), "init document service")

	svc, err := document.New("")
	require.NoError(t, err, "creating service")

	cleanup := func() {
		svc.Close()
		_ = os.Chdir(cwd)
		os.RemoveAll(tmpDir)
	}

	return svc, cleanup
}

func TestService_WriteRead(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/readme"
	content := "# Readme\nHello World"
	author := "tester"
	msg := "initial commit"

	require.NoError(t, svc.Write(ctx, path, content, author, msg))

	doc, err := svc.Latest(ctx, path, false)
	require.NoError(t, err)

	assert.Equal(t, content, doc.Content)
	assert.Equal(t, author, doc.Author)

	docs, err := svc.List(ctx, "", false, false)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, path, docs[0].Path)
}

func TestService_EditHistory(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	path := "notes/meeting"
	initial := "Meeting Notes\n- item 1"

	require.NoError(t, svc.Write(ctx, path, initial, "user1", "v1"))

	update := "Meeting Notes\n- item 1\n- item 2"
	require.NoError(t, svc.Write(ctx, path, update, "user2", "v2"))

	history, err := svc.History(ctx, path, 0, false)
	require.NoError(t, err)
	require.Len(t, history, 2)

	latest, err := svc.Latest(ctx, path, false)
	require.NoError(t, err)
	assert.Equal(t, update, latest.Content)
	assert.Equal(t, 2, latest.Version)

	v1, err := svc.Version(ctx, path, 1)
	require.NoError(t, err)
	assert.Equal(t, initial, v1.Content)
}

func TestService_DeleteRestore(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	path := "trash/me"
	content := "delete this"

	require.NoError(t, svc.Write(ctx, path, content, "user", "create"))

	_, err := svc.Latest(ctx, path, false)
	require.NoError(t, err, "should exist before delete")

	require.NoError(t, svc.Delete(ctx, path))

	_, err = svc.Latest(ctx, path, false)
	assert.Error(t, err, "should not find deleted doc by default")

	doc, err := svc.Latest(ctx, path, true)
	require.NoError(t, err, "should find deleted doc with includeDeleted=true")
	assert.NotNil(t, doc.DeletedAt, "deleted doc should have deleted_at set")

	require.NoError(t, svc.Restore(ctx, path))

	doc, err = svc.Latest(ctx, path, false)
	require.NoError(t, err, "should exist after restore")
	assert.Nil(t, doc.DeletedAt, "restored doc should not have deleted_at set")
}

func TestService_Tags(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	path := "docs/tagged"
	require.NoError(t, svc.Write(ctx, path, "tagged content", "user", "create"))

	opts := store.NewTagOptions()
	require.NoError(t, svc.Tag(ctx, path, "v1", opts))
	require.NoError(t, svc.Tag(ctx, path, "important", opts))

	tags, err := svc.ListTags(ctx, path, opts)
	require.NoError(t, err)
	assert.Len(t, tags, 2)

	allTags, err := svc.ListTags(ctx, "", opts)
	require.NoError(t, err)
	assert.Len(t, allTags, 2)

	paths, err := svc.PathsWithTag(ctx, "important", opts)
	require.NoError(t, err)
	assert.Equal(t, []string{path}, paths)

	require.NoError(t, svc.Untag(ctx, path, "v1", opts))

	tags, _ = svc.ListTags(ctx, path, opts)
	assert.Equal(t, []string{"important"}, tags)
}
