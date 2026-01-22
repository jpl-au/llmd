//go:build wasip1

package sdk

import (
	"context"
	"encoding/json"

	"github.com/jpl-au/llmd/proto/plugin"
)

// Register registers a plugin with the host.
//
// This function must be called from init(), not main(), because plugins
// are built with -buildmode=c-shared which creates a "reactor" WASM
// module where main() is not executed.
//
// Example:
//
//	func init() {
//	    sdk.Register(&MyPlugin{})
//	}
//
//	func main() {
//	    // Required for compilation but not called
//	}
func Register(p Plugin) {
	plugin.RegisterPlugin(&pluginAdapter{p: p})
}

// pluginAdapter adapts the SDK Plugin interface to the proto-generated interface.
type pluginAdapter struct {
	p Plugin
}

// Init implements plugin.Plugin.
func (a *pluginAdapter) Init(_ context.Context, _ *plugin.InitRequest) (*plugin.Manifest, error) {
	m := a.p.Manifest()
	return manifestToProto(m), nil
}

// ExecuteCommand implements plugin.Plugin.
func (a *pluginAdapter) ExecuteCommand(ctx context.Context, req *plugin.CommandRequest) (*plugin.CommandResponse, error) {
	sdkCtx := Context{
		Interface: Interface(req.GetContext().GetInterface()),
		Author:    req.GetContext().GetAuthor(),
		Format:    OutputFormat(req.GetContext().GetFormat()),
		Env:       req.GetContext().GetEnv(),
		Stdin:     req.GetStdin(),
	}

	flags := make(map[string]any)
	for k, v := range req.GetFlags() {
		flags[k] = v
	}

	result, err := a.p.ExecuteCommand(sdkCtx, req.GetCommand(), req.GetArgs(), flags)
	if err != nil {
		return &plugin.CommandResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resp := &plugin.CommandResponse{Success: true}
	switch r := result.(type) {
	case TextResult:
		resp.Output = string(r)
		resp.Format = plugin.OutputFormat_FORMAT_TEXT
	case JSONResult:
		data, _ := json.Marshal(r.Data)
		resp.Output = string(data)
		resp.Format = plugin.OutputFormat_FORMAT_JSON
	}

	return resp, nil
}

// HandleEvent implements plugin.Plugin.
func (a *pluginAdapter) HandleEvent(ctx context.Context, e *plugin.Event) (*plugin.Empty, error) {
	handler, ok := a.p.(EventHandler)
	if !ok {
		return &plugin.Empty{}, nil
	}

	metadata := make(map[string]any)
	for k, v := range e.GetMetadata() {
		metadata[k] = v
	}

	sdkEvent := Event{
		Type:      e.GetType(),
		Path:      e.GetPath(),
		Version:   int(e.GetVersion()),
		Author:    e.GetAuthor(),
		Timestamp: e.GetTimestamp(),
		Metadata:  metadata,
	}

	if err := handler.HandleEvent(sdkEvent); err != nil {
		return nil, err
	}
	return &plugin.Empty{}, nil
}

// Shutdown implements plugin.Plugin.
func (a *pluginAdapter) Shutdown(_ context.Context, _ *plugin.Empty) (*plugin.Empty, error) {
	if s, ok := a.p.(Shutdowner); ok {
		if err := s.Shutdown(); err != nil {
			return nil, err
		}
	}
	return &plugin.Empty{}, nil
}

// manifestToProto converts an SDK Manifest to the proto-generated type.
func manifestToProto(m Manifest) *plugin.Manifest {
	commands := make([]*plugin.Command, len(m.Commands))
	for i, cmd := range m.Commands {
		flags := make([]*plugin.Flag, len(cmd.Flags))
		for j, f := range cmd.Flags {
			flags[j] = &plugin.Flag{
				Name:        f.Name,
				Short:       f.Short,
				Type:        f.Type,
				Default:     f.Default,
				Description: f.Description,
				Required:    f.Required,
			}
		}
		commands[i] = &plugin.Command{
			Name:        cmd.Name,
			Description: cmd.Description,
			Usage:       cmd.Usage,
			Flags:       flags,
			McpEnabled:  cmd.MCPEnabled,
			McpName:     cmd.MCPName,
		}
	}

	return &plugin.Manifest{
		Name:             m.Name,
		Version:          m.Version,
		Author:           m.Author,
		Description:      m.Description,
		MinHostVersion:   m.MinHostVersion,
		Commands:         commands,
		SubscribedEvents: m.SubscribedEvents,
	}
}
