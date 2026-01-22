// Package host provides CLI generation from plugin manifests.
package host

// BuildCLI generates CLI commands from loaded plugin manifests.
// Uses urfave/cli v3 to dynamically create commands.
func (h *Host) BuildCLI() error {
	// TODO: Import urfave/cli/v3
	// TODO: For each plugin, get manifest
	// TODO: For each command in manifest, create cli.Command
	// TODO: Wire Action to h.ExecuteCommand
	return nil
}

// Example of what the generated CLI will look like:
//
// func buildCommands(h *Host) []*cli.Command {
//     var commands []*cli.Command
//     for _, p := range h.plugins {
//         manifest := p.Manifest()
//         for _, cmd := range manifest.Commands {
//             commands = append(commands, &cli.Command{
//                 Name:  cmd.Name,
//                 Usage: cmd.Description,
//                 Flags: buildFlags(cmd.Flags),
//                 Action: func(ctx *cli.Context) error {
//                     return h.ExecuteCommand(cmd.Name, ctx.Args().Slice(), extractFlags(ctx))
//                 },
//             })
//         }
//     }
//     return commands
// }
