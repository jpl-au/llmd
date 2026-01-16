package extension

import (
	"testing"

	"github.com/spf13/cobra"
)

// testExtension is a minimal Extension implementation for testing.
type testExtension struct {
	name string
}

func (e testExtension) Name() string               { return e.name }
func (e testExtension) Commands() []*cobra.Command { return nil }
func (e testExtension) MCPTools() []MCPTool        { return nil }

func TestRegister_PanicOnDuplicate(t *testing.T) {
	// Register with a unique name for this test
	name := "test-duplicate-panic"
	Register(testExtension{name: name})

	// Registering the same name again should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration, got none")
		}
	}()

	Register(testExtension{name: name})
}
