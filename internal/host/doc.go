// Package host provides the plugin host for llmd.
//
// The host manages plugin registration and command execution. It connects
// plugins to the document store and routes commands to the appropriate plugin.
//
// # Architecture
//
// The host loads plugins from two sources:
//
//  1. Compiled extensions — registered at init-time via [extension.Register]
//  2. Yaegi dynamic plugins — Go source loaded at runtime from plugin directories
//
// Commands are routed by name to the plugin that registered them.
//
// # Creating a Host
//
// Create a host with a store for full functionality:
//
//	store, _ := llmd.Open("")
//	h := host.New(store)
//
//	result, err := h.Exec("ls", nil, "", nil)
//
// Or without a store for discovery (help text, command listing):
//
//	h := host.New(nil)
//	for name, cmd := range h.Commands() {
//	    fmt.Printf("%s: %s\n", name, cmd.Desc)
//	}
//
// # Store API
//
// The host sets [sdk.API] to an implementation that wraps the llmd store.
// This allows plugins to access documents without direct store dependencies:
//
//	// In a plugin command:
//	content, _ := sdk.API.Read("path/to/doc.md", 0)
//	sdk.API.Write("path/to/doc.md", content, author, message)
//
// # Adding Compiled Plugins
//
// Compiled plugins are discovered via the extension registry:
//
//  1. Create a package implementing [sdk.Plugin]
//  2. Wrap it in an [extension.Extension] and call [extension.Register] in init()
//  3. Import the package (blank import) in main.go
//
// # Yaegi Dynamic Plugins
//
// Yaegi plugins are Go source files loaded at runtime. Plugin directories
// are searched in order (local overrides global):
//
//  1. .llmd/plugins/<name>/ — project-local plugins
//  2. ~/.llmd/plugins/<name>/ — global plugins
//
// Each plugin directory must contain .go files with a New() function
// returning an [sdk.Plugin]. A broken plugin is logged and skipped.
package host
