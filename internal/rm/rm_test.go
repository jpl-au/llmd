package rm_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/rm"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupService creates a temporary service and returns it along with a cleanup function.
func setupService(t *testing.T) (service.Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "llmd-rm-test-*")
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

func TestRun_ResolvesKeyToPath(t *testing.T) {
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

	// Call rm.Run with the key instead of the path
	var buf bytes.Buffer
	result, err := rm.Run(ctx, &buf, svc, key, rm.Options{})
	require.NoError(t, err)

	// Result.Path should be the resolved document path, not the key
	assert.Equal(t, docPath, result.Path, "Result.Path should be the resolved document path, not the key")
	assert.NotEqual(t, key, result.Path, "Result.Path should not be the key")
	assert.Contains(t, result.Deleted, docPath)
}

func TestRun_WithPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a document
	docPath := "docs/readme"
	require.NoError(t, svc.Write(ctx, docPath, "content", "tester", "initial"))

	// Call rm.Run with the path directly
	var buf bytes.Buffer
	result, err := rm.Run(ctx, &buf, svc, docPath, rm.Options{})
	require.NoError(t, err)

	// Result.Path should be the document path
	assert.Equal(t, docPath, result.Path)
	assert.Contains(t, result.Deleted, docPath)
}
