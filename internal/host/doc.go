// Package host provides the plugin host for llmd.
//
// The host manages plugin registration and command execution. It connects
// plugins to the document store and routes commands to the appropriate plugin.
//
// # Architecture
//
// The host uses a simple native plugin model:
//
//   - Plugins are compiled directly into the binary (no WASM, no interpreters)
//   - The core plugin is registered automatically when [New] is called
//   - Commands are routed by name to the plugin that registered them
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
// # Adding Plugins
//
// Currently, plugins are registered in [New]. To add a new plugin:
//
//  1. Create a package implementing [sdk.Plugin]
//  2. Import it in host.go
//  3. Call h.register(myplugin.New()) in [New]
//
// Future versions may support dynamic plugin loading via Yaegi.
package host
