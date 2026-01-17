package tag_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/tag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupService creates a temporary service and returns it along with a cleanup function.
func setupService(t *testing.T) (service.Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "llmd-tag-test-*")
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

func TestAdd_ResolvesKeyToPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a document
	docPath := "docs/readme"
	require.NoError(t, svc.Write(ctx, docPath, "content", "tester", "initial"))

	// Get the document to find its key
	doc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	key := doc.Key

	// Call tag.Add with the key instead of the path
	var buf bytes.Buffer
	result, err := tag.Add(ctx, &buf, svc, key, "important")
	require.NoError(t, err)

	// Result.Path should be the resolved document path, not the key
	assert.Equal(t, docPath, result.Path, "Result.Path should be the resolved document path, not the key")
	assert.NotEqual(t, key, result.Path, "Result.Path should not be the key")
}

func TestRemove_ResolvesKeyToPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a document and add a tag
	docPath := "docs/readme"
	require.NoError(t, svc.Write(ctx, docPath, "content", "tester", "initial"))

	var buf bytes.Buffer
	_, err := tag.Add(ctx, &buf, svc, docPath, "important")
	require.NoError(t, err)

	// Get the document to find its key
	doc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	key := doc.Key

	// Call tag.Remove with the key instead of the path
	buf.Reset()
	result, err := tag.Remove(ctx, &buf, svc, key, "important")
	require.NoError(t, err)

	// Result.Path should be the resolved document path, not the key
	assert.Equal(t, docPath, result.Path, "Result.Path should be the resolved document path, not the key")
	assert.NotEqual(t, key, result.Path, "Result.Path should not be the key")
}

func TestList_ResolvesKeyToPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a document and add a tag
	docPath := "docs/readme"
	require.NoError(t, svc.Write(ctx, docPath, "content", "tester", "initial"))

	var buf bytes.Buffer
	_, err := tag.Add(ctx, &buf, svc, docPath, "important")
	require.NoError(t, err)

	// Get the document to find its key
	doc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	key := doc.Key

	// Call tag.List with the key instead of the path
	buf.Reset()
	result, err := tag.List(ctx, &buf, svc, key)
	require.NoError(t, err)

	// Result.Path should be the resolved document path, not the key
	assert.Equal(t, docPath, result.Path, "Result.Path should be the resolved document path, not the key")
	assert.NotEqual(t, key, result.Path, "Result.Path should not be the key")
	assert.Contains(t, result.Tags, "important")
}

func TestList_EmptyPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create documents and add tags
	require.NoError(t, svc.Write(ctx, "docs/a", "content a", "tester", "initial"))
	require.NoError(t, svc.Write(ctx, "docs/b", "content b", "tester", "initial"))

	var buf bytes.Buffer
	_, err := tag.Add(ctx, &buf, svc, "docs/a", "tag1")
	require.NoError(t, err)
	_, err = tag.Add(ctx, &buf, svc, "docs/b", "tag2")
	require.NoError(t, err)

	// Call tag.List with empty path to list all tags
	buf.Reset()
	result, err := tag.List(ctx, &buf, svc, "")
	require.NoError(t, err)

	// Result.Path should be empty (no resolution needed)
	assert.Equal(t, "", result.Path)
	assert.Len(t, result.Tags, 2)
}
