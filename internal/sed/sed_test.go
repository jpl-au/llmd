package sed_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/sed"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupService creates a temporary service and returns it along with a cleanup function.
func setupService(t *testing.T) (service.Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "llmd-sed-test-*")
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
	require.NoError(t, svc.Write(ctx, docPath, "hello world", "tester", "initial"))

	// Get the document to find its key
	doc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	key := doc.Key

	// Call sed.Run with the key instead of the path
	var buf bytes.Buffer
	result, err := sed.Run(ctx, &buf, svc, key, "s/hello/goodbye/", sed.Options{Author: "tester"})
	require.NoError(t, err)

	// Result.Path should be the resolved document path, not the key
	assert.Equal(t, docPath, result.Path, "Result.Path should be the resolved document path, not the key")
	assert.NotEqual(t, key, result.Path, "Result.Path should not be the key")

	// Verify the content was changed
	updatedDoc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	assert.Equal(t, "goodbye world", updatedDoc.Content)
}

func TestRun_WithPath(t *testing.T) {
	svc, cleanup := setupService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a document
	docPath := "docs/readme"
	require.NoError(t, svc.Write(ctx, docPath, "hello world", "tester", "initial"))

	// Call sed.Run with the path directly
	var buf bytes.Buffer
	result, err := sed.Run(ctx, &buf, svc, docPath, "s/hello/goodbye/", sed.Options{Author: "tester"})
	require.NoError(t, err)

	// Result.Path should be the document path
	assert.Equal(t, docPath, result.Path)

	// Verify the content was changed
	updatedDoc, err := svc.Latest(ctx, docPath, false)
	require.NoError(t, err)
	assert.Equal(t, "goodbye world", updatedDoc.Content)
}

func TestParseExpr(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		old     string
		new     string
		global  bool
		wantErr bool
	}{
		{
			name: "simple substitution",
			expr: "s/old/new/",
			old:  "old",
			new:  "new",
		},
		{
			name:   "global substitution",
			expr:   "s/old/new/g",
			old:    "old",
			new:    "new",
			global: true,
		},
		{
			name: "alternate delimiter",
			expr: "s|old|new|",
			old:  "old",
			new:  "new",
		},
		{
			name: "empty replacement",
			expr: "s/delete//",
			old:  "delete",
			new:  "",
		},
		{
			name:    "invalid command",
			expr:    "d/old/new/",
			wantErr: true,
		},
		{
			name:    "too short",
			expr:    "s//",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sed.ParseExpr(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.old, result.Old)
			assert.Equal(t, tt.new, result.New)
			assert.Equal(t, tt.global, result.Global)
		})
	}
}
