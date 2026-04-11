// Package sdk provides the extension SDK for llmd.
//
// Extensions implement the [Extension] interface to provide commands. The
// host loads extensions and routes commands to them. Extensions access
// the store through domain-specific globals: [Documents], [Tasks],
// [Links], and [Tags]. Each global is a focused interface with
// unprefixed methods.
//
// # Writing an Extension
//
// An extension is a type that implements three methods:
//
//	type MyExt struct{}
//
//	func (e *MyExt) Name() string { return "myext" }
//
//	func (e *MyExt) Commands() []sdk.Command {
//	    return []sdk.Command{
//	        {Name: "hello", Desc: "Say hello", Usage: "hello <name>"},
//	    }
//	}
//
//	func (e *MyExt) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
//	    switch cmd {
//	    case "hello":
//	        if len(args) == 0 {
//	            return nil, fmt.Errorf("hello: %w", sdk.ErrMissingArg)
//	        }
//	        return sdk.Text(fmt.Sprintf("Hello, %s!", args[0])), nil
//	    default:
//	        return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
//	    }
//	}
//
// # Accessing the Store
//
// Commands access the store through domain globals:
//
//	content, err := sdk.Documents.Read("notes/todo.md", sdk.ReadOpts{})  // latest
//	if err != nil {
//	    return nil, err
//	}
//
//	err = sdk.Documents.Write("notes/new.md", []byte("# New"), sdk.WriteOpts{Author: ctx.Author, Message: "initial"})
//
//	err = sdk.Tags.Add("notes/todo.md", "important", ctx.Author)
//
//	err = sdk.Links.Add("notes/a", "notes/b", "related", ctx.Author)
//
//	task, err := sdk.Tasks.Add("Fix auth bug", body, sdk.TaskAddOpts{Author: ctx.Author})
//
// # Returning Results
//
// Commands return one of three result types:
//
//   - [Text]: Plain text output
//   - [Data]: Structured data (always output as JSON)
//   - [Result]: Both text and structured data (text for terminal, data for --json)
//
// Example:
//
//	// Text only
//	return sdk.Text("Done"), nil
//
//	// Structured data only
//	return sdk.Data{V: myStruct}, nil
//
//	// Both (text for humans, data for machines)
//	return sdk.Result{Text: "Found 5 documents", Data: docs}, nil
//
// # Command Flags
//
// Commands declare their flags in the [Command] struct. The host does not
// parse flags; commands receive raw args and parse them directly. This
// allows commands to follow Unix conventions (e.g., combined short flags
// like -la).
//
//	{Name: "ls", Flags: []sdk.Flag{
//	    {Name: "l", Type: "bool", Desc: "Long format"},
//	    {Name: "a", Type: "bool", Desc: "Include deleted"},
//	}}
package sdk
