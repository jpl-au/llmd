// This file implements event-related host functions.
//
// Events allow plugins to be notified of changes to the document store
// asynchronously. Plugins declare their event subscriptions in their manifest
// and receive events through the HandleEvent callback.
//
// Note: Event subscriptions and emissions are not fully implemented yet.
// These stubs are provided to satisfy the proto interface.
package host

import (
	"context"

	hostpb "github.com/jpl-au/llmd/proto/host"
)

// EventSubscribe subscribes to events.
//
// This is currently a no-op as event subscription is handled through the
// plugin manifest's SubscribedEvents field. Direct subscriptions may be
// supported in future versions.
func (h *HostFuncs) EventSubscribe(ctx context.Context, req *hostpb.SubscribeRequest) (*hostpb.Empty, error) {
	return &hostpb.Empty{}, nil
}

// EventEmit emits an event to subscribers.
//
// This is currently a no-op as the event bus infrastructure is not yet
// implemented. In future versions, this will allow plugins to emit custom
// events that other plugins can subscribe to.
func (h *HostFuncs) EventEmit(ctx context.Context, req *hostpb.EmitRequest) (*hostpb.Empty, error) {
	return &hostpb.Empty{}, nil
}
