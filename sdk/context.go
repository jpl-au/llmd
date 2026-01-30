//go:build wasip1

package sdk

// Context provides execution context to commands.
//
// The Context is passed to ExecuteCommand and provides information about
// how and by whom the command was invoked. Commands can use this to
// customise their behaviour (e.g., different output for CLI vs MCP).
type Context struct {
	// Interface indicates how the command was invoked (CLI, MCP, or API).
	Interface Interface

	// Author identifies who is executing the command.
	// Recorded in version history for auditing.
	Author string

	// Env contains environment variables passed to the command.
	// Only populated if the host allows environment access.
	Env map[string]string

	// Stdin contains piped input data from the host.
	// For CLI: content piped to the command (e.g., echo "data" | llmd write).
	// For MCP: content passed via tool parameters.
	Stdin []byte
}

// Interface indicates how the command was invoked.
type Interface int

const (
	// InterfaceCLI indicates command-line invocation.
	InterfaceCLI Interface = iota

	// InterfaceMCP indicates Model Context Protocol invocation.
	InterfaceMCP

	// InterfaceAPI indicates HTTP/gRPC API invocation.
	InterfaceAPI
)

