package documents_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	s, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func testDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func testRestoreOpts() documents.RestoreOptions {
	return documents.RestoreOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func testEditOpts() documents.EditOptions {
	return documents.EditOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func testMoveOpts() documents.MoveOptions {
	return documents.MoveOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func testCopyOpts() documents.CopyOptions {
	return documents.CopyOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}
