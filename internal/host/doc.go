// Package host provides the command host for llmd.
//
// The host manages extension registration and command execution. It
// connects extensions to the document store and routes commands to the
// appropriate extension.
//
// # Architecture
//
// The host loads compiled extensions registered at init-time via
// [extension.Register]. Commands are routed by name to the extension
// that registered them.
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
// [sdk.Links], [sdk.Tags]) that wrap the llmd store. This allows
// extensions to access the store without direct dependencies:
//
//	// In an extension command:
//	content, _ := sdk.Documents.Read("path/to/doc.md", sdk.ReadOpts{})
//	sdk.Documents.Write("path/to/doc.md", content, sdk.WriteOpts{Author: author, Message: message})
//	sdk.Tags.Add("path/to/doc.md", "important", author)
//
// # Adding Extensions
//
// Extensions are discovered via the extension registry:
//
//  1. Create a package implementing [sdk.Extension]
//  2. Call [extension.Register] in init()
//  3. Import the package (blank import) in main.go
package host
