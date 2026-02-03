// Package sdk provides the plugin SDK for llmd.
//
// Plugins implement the [Plugin] interface to provide commands. The host
// loads plugins and routes commands to them. Plugins access the document
// store through the global [API] variable, which is set by the host before
// command execution.
//
// # Writing a Plugin
//
// A plugin is a type that implements three methods:
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Name() string { return "myplugin" }
//
//	func (p *MyPlugin) Commands() []sdk.Command {
//	    return []sdk.Command{
//	        {Name: "hello", Desc: "Say hello", Usage: "hello <name>"},
//	    }
//	}
//
//	func (p *MyPlugin) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Result, error) {
//	    switch cmd {
//	    case "hello":
//	        if len(args) == 0 {
//	            return nil, fmt.Errorf("hello: missing name")
//	        }
//	        return sdk.Text(fmt.Sprintf("Hello, %s!", args[0])), nil
//	    default:
//	        return nil, fmt.Errorf("unknown command: %s", cmd)
//	    }
//	}
//
// # Accessing the Store
//
// Commands access the document store through [API]:
//
//	content, err := sdk.API.Read("notes/todo.md", 0)  // 0 = latest version
//	if err != nil {
//	    return nil, err
//	}
//
//	err = sdk.API.Write("notes/new.md", []byte("# New"), ctx.Author, "initial")
//
// # Returning Results
//
// Commands return one of three result types:
//
//   - [Text]: Plain text output
//   - [Data]: Structured data (always output as JSON)
//   - [Rich]: Both text and structured data (text for terminal, data for --json)
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
//	return sdk.Rich{Text: "Found 5 documents", Data: docs}, nil
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
