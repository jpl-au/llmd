// webhook.go broadcasts store events to configured webhook endpoints.
// Each event is POSTed as JSON to every endpoint. Delivery is
// fire-and-forget: errors are logged at Warn level and never block
// the event bus.
package server

import (
	"encoding/json"
	"log/slog"

	client "github.com/jpl-au/http-client"
	"github.com/jpl-au/http-client/options"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
)

// webhookHub dispatches events to external HTTP endpoints.
type webhookHub struct {
	endpoints []webhookEndpoint
}

// webhookEndpoint is a resolved webhook destination.
type webhookEndpoint struct {
	name string
	url  string
	opts *options.Option
}

// newWebhookHub creates a hub from the webhook configuration map.
// Returns nil if no webhook endpoints are configured.
func newWebhookHub(webhooks map[string]config.WebhookConfig) *webhookHub {
	if len(webhooks) == 0 {
		return nil
	}

	h := &webhookHub{}
	for name, wh := range webhooks {
		opt := options.New().
			AddHeader("Content-Type", "application/json")
		if wh.Key != "" {
			opt.AddHeader("Authorization", "Bearer "+wh.Key)
		}

		h.endpoints = append(h.endpoints, webhookEndpoint{
			name: name,
			url:  wh.URL,
			opts: opt,
		})
	}
	return h
}

// Broadcast sends an event to all configured webhook endpoints.
// Called synchronously from the event bus — dispatches are
// non-blocking via goroutines.
func (h *webhookHub) Broadcast(e events.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		slog.Warn("webhook: marshalling event", "err", err)
		return
	}

	for _, ep := range h.endpoints {
		go post(ep, data)
	}
}

// post sends the event payload to a single endpoint.
func post(ep webhookEndpoint, data []byte) {
	resp, err := client.Post(ep.url, data, ep.opts)
	if err != nil {
		slog.Warn("webhook: delivery failed", "name", ep.name, "url", ep.url, "err", err)
		return
	}
	if resp.StatusCode >= 400 {
		slog.Warn("webhook: non-success response", "name", ep.name, "url", ep.url, "status", resp.StatusCode)
	}
}
