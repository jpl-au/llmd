package cli

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestHandlersMatchCommands checks that every command in Commands()
// has a handler and every handler has a command, and that an unknown
// name is reported as sdk.ErrUnknownCmd.
func TestHandlersMatchCommands(t *testing.T) {
	specs := make(map[string]bool)
	for _, cmd := range (&CLI{}).Commands() {
		specs[cmd.Name] = true
		if handlers[cmd.Name] == nil {
			t.Errorf("command %q has no handler", cmd.Name)
		}
	}
	for name := range handlers {
		if !specs[name] {
			t.Errorf("handler %q has no command spec", name)
		}
	}

	_, err := (&CLI{}).Exec(sdk.Context{}, "no-such-command", nil)
	if !errors.Is(err, sdk.ErrUnknownCmd) {
		t.Errorf("Exec(no-such-command) error = %v, want sdk.ErrUnknownCmd", err)
	}
}
