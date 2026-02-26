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
// Open a host with a store for full functionality:
//
//	h, _ := host.Open("")
//	defer h.Close()
//
//	result, err := h.Exec("ls", nil, "", nil, "")
//
// Or without a store for discovery (help text, command listing):
//
//	h := host.New()
//	for name, cmd := range h.Commands() {
//	    fmt.Printf("%s: %s\n", name, cmd.Desc)
//	}
//
// # Store API
//
// The host sets domain-specific globals ([sdk.Documents], [sdk.Tasks],
// [sdk.Links], [sdk.Tags]) that wrap the llmd store. This allows plugins
// to access the store without direct dependencies:
//
//	// In a plugin command:
//	content, _ := sdk.Documents.Read("path/to/doc.md", 0)
//	sdk.Documents.Write("path/to/doc.md", content, author, message)
//	sdk.Tags.Add("path/to/doc.md", "important", author)
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
