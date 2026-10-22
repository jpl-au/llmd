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
// Options adjust the defaults, for example to use a different git
// implementation or fixed validation limits:
//
//	h, _ := host.Open("", host.WithGit(fakeGit), host.WithLimits(limits))
//	for name, cmd := range h.Commands() {
//	    fmt.Printf("%s: %s\n", name, cmd.Desc)
//	}
//
// # Store API
//
// For every request the host builds an [sdk.Context] carrying store
// bridges ([sdk.DocumentStore], [sdk.TaskStore], [sdk.LinkStore],
// [sdk.TagStore], ...) bound to that request's context. Extensions
// access the store through it without direct dependencies:
//
//	// In an extension command:
//	content, _ := ctx.Documents.Read("path/to/doc.md", sdk.ReadOpts{})
//	ctx.Documents.Write("path/to/doc.md", content, sdk.WriteOpts{Author: ctx.Author, Message: message})
//	ctx.Tags.Add("path/to/doc.md", "important", ctx.Author)
//
// # Adding Extensions
//
// Extensions are discovered via the extension registry:
//
//  1. Create a package implementing [sdk.Extension]
//  2. Call [extension.Register] in init()
//  3. Import the package (blank import) in main.go
package host
