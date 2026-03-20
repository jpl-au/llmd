// sse.go provides a Server-Sent Events endpoint for real-time event
// streaming. Connected clients receive store events (document writes,
// audit creation, task moves, etc.) as they happen.
//
// Usage:
//
//	GET /events                        Stream all events
//	GET /events?type=audit.created     Stream specific event types
//	GET /events?type=audit.created,task.moved  Multiple types
//
// Events are delivered in standard SSE format:
//
//	event: audit.created
//	data: {"type":"audit.created","path":"docs/spec","key":"abc","author":"Gemini"}
//
// The hub fans out events from the internal bus to all connected
// clients. Slow clients that fall behind have events dropped rather
// than blocking the bus.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/jpl-au/llmd/pkg/events"
)

// sseHub manages connected SSE clients and fans out events from
// the internal bus. It implements a non-blocking broadcast: if a
// client's buffer is full, the event is dropped for that client.
type sseHub struct {
	mu      sync.RWMutex
	clients map[*sseClient]struct{}
}

// sseClient represents a connected SSE consumer. Events matching
// the client's type filter are sent to its channel.
type sseClient struct {
	events chan events.Event
	types  map[string]bool // nil = all events
	done   chan struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[*sseClient]struct{})}
}

// Broadcast sends an event to all connected clients whose filters
// match. Called synchronously from the event bus - must not block.
func (h *sseHub) Broadcast(e events.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.types != nil && !c.types[e.Type] {
			continue
		}
		select {
		case c.events <- e:
		default:
			slog.Debug("sse: dropping event for slow client", "type", e.Type)
		}
	}
}

// ServeHTTP handles GET /events. It upgrades the connection to an
// SSE stream and sends events until the client disconnects.
func (h *sseHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Parse type filter from query params.
	var types map[string]bool
	if t := r.URL.Query().Get("type"); t != "" {
		types = make(map[string]bool)
		for v := range strings.SplitSeq(t, ",") {
			types[strings.TrimSpace(v)] = true
		}
	}

	client := &sseClient{
		events: make(chan events.Event, 64),
		types:  types,
		done:   make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	slog.Debug("sse: client connected", "types", types)

	for {
		select {
		case e := <-client.events:
			data, err := json.Marshal(e)
			if err != nil {
				slog.Warn("sse: marshalling event", "err", err)
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			slog.Debug("sse: client disconnected")
			return
		}
	}
}
