// Package host provides MCP server functionality.
package host

// ServeMCP starts the MCP server on stdio.
// Tools are generated from plugin command manifests.
func (h *Host) ServeMCP() error {
	// TODO: Import mcp-go
	// TODO: For each plugin command with MCPEnabled=true:
	//   - Generate tool schema from command flags
	//   - Register tool handler that calls h.ExecuteCommand
	// TODO: Start stdio server
	return nil
}

// Example of MCP tool registration:
//
// func (h *Host) registerMCPTools(server *mcp.Server) {
//     for _, p := range h.plugins {
//         manifest := p.Manifest()
//         for _, cmd := range manifest.Commands {
//             if !cmd.MCPEnabled {
//                 continue
//             }
//             toolName := cmd.MCPName
//             if toolName == "" {
//                 toolName = "llmd_" + cmd.Name
//             }
//             server.AddTool(mcp.Tool{
//                 Name:        toolName,
//                 Description: cmd.Description,
//                 InputSchema: buildJSONSchema(cmd.Flags),
//             }, func(args map[string]any) (any, error) {
//                 return h.ExecuteCommand(cmd.Name, nil, args)
//             })
//         }
//     }
// }
