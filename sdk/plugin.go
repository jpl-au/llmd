//go:build wasip1

package sdk

// Plugin is the interface that all llmd plugins must implement.
//
// Every plugin must implement this interface to be loaded by the host.
// The interface provides metadata about the plugin through Manifest()
// and handles command execution through ExecuteCommand().
//
// Plugins may optionally implement additional interfaces:
//   - EventHandler: To receive notifications about document store events
//   - Shutdowner: To perform cleanup when the plugin is unloaded
type Plugin interface {
	// Manifest returns the plugin's metadata and command definitions.
	// Called once when the plugin is initialised and should return
	// static information about the plugin's capabilities.
	Manifest() Manifest

	// ExecuteCommand executes a command and returns the result.
	// The cmd parameter is the command name as registered in the manifest.
	// The args parameter contains positional arguments passed to the command.
	// The flags parameter contains named flags with their parsed values.
	// Returns a Result (TextResult or JSONResult) on success, or an error.
	ExecuteCommand(ctx Context, cmd string, args []string, flags map[string]any) (Result, error)
}

// EventHandler is an optional interface for plugins that handle events.
//
// Plugins that implement this interface receive notifications when events
// they have subscribed to occur. Event subscriptions are declared in the
// Manifest's SubscribedEvents field.
//
// Events are delivered asynchronously and should be handled quickly to
// avoid blocking other operations.
type EventHandler interface {
	// HandleEvent is called when a subscribed event occurs.
	// Returning an error logs the failure but won't affect other handlers.
	HandleEvent(event Event) error
}

// Shutdowner is an optional interface for plugins that need cleanup.
//
// Plugins that allocate resources (file handles, network connections,
// goroutines) should implement this interface to ensure proper cleanup
// when the host shuts down or the plugin is unloaded.
type Shutdowner interface {
	// Shutdown is called when the plugin is being unloaded.
	// The host waits for this method to return before unloading.
	Shutdown() error
}
