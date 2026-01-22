//go:build wasip1

package sdk

// Manifest describes a plugin and its capabilities.
//
// The Manifest is returned by Plugin.Manifest() and provides all the
// information the host needs to integrate the plugin. This includes
// metadata (name, version), the commands the plugin provides, and
// which events it wants to receive.
type Manifest struct {
	// Name is the unique identifier for this plugin.
	// Should be lowercase, alphanumeric with hyphens (e.g., "my-plugin").
	Name string `json:"name"`

	// Version is the plugin version in semantic versioning format.
	Version string `json:"version"`

	// Author is the plugin author's name or organisation.
	Author string `json:"author,omitempty"`

	// Description is a brief summary of what the plugin does.
	Description string `json:"description,omitempty"`

	// MinHostVersion is the minimum llmd host version required.
	// If specified, the host refuses to load incompatible plugins.
	MinHostVersion string `json:"min_host_version,omitempty"`

	// Commands lists all CLI/MCP commands this plugin provides.
	Commands []Command `json:"commands"`

	// SubscribedEvents lists event types this plugin wants to receive.
	// Use the Event* constants to subscribe.
	SubscribedEvents []string `json:"subscribed_events,omitempty"`
}

// Command defines a CLI/MCP command provided by a plugin.
//
// Commands can be invoked from the command line (llmd <command>) or via
// MCP (Model Context Protocol) when MCPEnabled is true.
type Command struct {
	// Name is the command name as typed on the command line.
	// Should be lowercase, short, and memorable.
	Name string `json:"name"`

	// Description is a one-line summary shown in help text.
	Description string `json:"description"`

	// Usage shows the command syntax (e.g., "cat <path>").
	Usage string `json:"usage,omitempty"`

	// Flags lists optional flags the command accepts.
	Flags []Flag `json:"flags,omitempty"`

	// MCPEnabled controls whether this command is exposed as an MCP tool.
	MCPEnabled bool `json:"mcp_enabled"`

	// MCPName overrides the MCP tool name (defaults to the command name).
	MCPName string `json:"mcp_name"`
}

// Flag defines a command flag.
//
// Flags provide optional parameters to commands. They can be specified
// with either the long form (--name) or short form (-n if defined).
type Flag struct {
	// Name is the flag name without dashes (e.g., "version" for --version).
	Name string `json:"name"`

	// Short is the single-character shorthand (e.g., "v" for -v).
	Short string `json:"short,omitempty"`

	// Type specifies the value type: "string", "int", "bool", or "stringSlice".
	Type string `json:"type"`

	// Default is the default value if the flag is not specified.
	Default string `json:"default,omitempty"`

	// Description is shown in help text for this flag.
	Description string `json:"description"`

	// Required marks this flag as mandatory.
	Required bool `json:"required,omitempty"`
}
