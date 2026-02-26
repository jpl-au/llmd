// events.go bridges the internal event bus to extension EventHandlers.
//
// The internal bus (internal/llmd/events) fires pkg/events.Event structs
// for document mutations. Extensions use a different event hierarchy
// (extension.Event interface with concrete types). This bridge subscribes
// to the internal bus and converts events into the extension format,
// dispatching to all extensions that implement extension.EventHandler.
//
// Only document events are bridged for now — tag and link events are not
// yet emitted by the internal bus.

package host

import (
	"context"

	"github.com/jpl-au/llmd/extension"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// eventBridge forwards internal bus events to extension EventHandlers.
// It implements internal/llmd/events.Handler.
type eventBridge struct {
	handlers []extension.EventHandler
	ctx      extension.Context
}

// HandleEvent converts an internal event to the extension format and
// dispatches it to all registered extension handlers. Unrecognised
// event types are silently ignored.
func (b *eventBridge) HandleEvent(_ context.Context, e pkgevents.Event) error {
	ext := toExtEvent(e)
	if ext == nil {
		return nil
	}
	for _, h := range b.handlers {
		if err := h.HandleEvent(b.ctx, ext); err != nil {
			return err
		}
	}
	return nil
}

// toExtEvent maps a pkg/events.Event to the corresponding extension
// event type. Returns nil for event types that have no extension
// counterpart.
func toExtEvent(e pkgevents.Event) extension.Event {
	switch e.Type {
	case pkgevents.DocumentWritten:
		return extension.DocumentWriteEvent{
			Path:    e.Path,
			Version: e.Version,
			Author:  e.Author,
		}
	case pkgevents.DocumentDeleted:
		return extension.DocumentDeleteEvent{
			Path: e.Path,
		}
	case pkgevents.DocumentRestored:
		return extension.DocumentRestoreEvent{
			Path:    e.Path,
			Version: e.Version,
		}
	case pkgevents.DocumentMoved:
		var oldPath string
		if m := e.Metadata; m != nil {
			if v, ok := m["old_path"].(string); ok {
				oldPath = v
			}
		}
		return extension.DocumentMoveEvent{
			Path:    e.Path,
			OldPath: oldPath,
			Version: e.Version,
		}
	default:
		return nil
	}
}
