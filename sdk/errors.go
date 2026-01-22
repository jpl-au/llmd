//go:build wasip1

package sdk

// ErrUnknownCommand is returned when a plugin receives an unknown command.
//
// Return this error from ExecuteCommand when the command name doesn't
// match any commands declared in the plugin's Manifest.
type ErrUnknownCommand struct {
	Command string
}

func (e ErrUnknownCommand) Error() string {
	return "unknown command: " + e.Command
}
