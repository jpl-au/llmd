// webhook.go delivers store events to configured HTTP endpoints.
//
// Deliveries are queued and sent by a fixed set of workers, so a slow
// endpoint cannot pile up goroutines. Each delivery is retried with
// backoff after a network error or a 5xx response, is not retried
// after a 4xx (the request itself was rejected), and is dropped with a
// warning after the last attempt or when the queue is full. The event
// bus never waits on delivery.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"

	client "github.com/jpl-au/http-client"
	"github.com/jpl-au/http-client/options"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
)

// Delivery defaults. Retries and timeout can be set per endpoint in
// the configuration; workers and queue depth are fixed.
const (
	webhookWorkers = 4
	webhookQueue   = 256
	webhookRetries = 2 // attempts after the first, so three in all
	webhookTimeout = 10 * time.Second
)

// webhookEndpoint is a resolved webhook destination.
type webhookEndpoint struct {
	name     string
	url      string
	key      string
	attempts int           // total attempts per delivery
	timeout  time.Duration // per request
}

// delivery is one event bound for one endpoint.
type delivery struct {
	ep   webhookEndpoint
	data []byte
}

// webhookHub dispatches events to external HTTP endpoints.
type webhookHub struct {
	endpoints []webhookEndpoint
	queue     chan delivery
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	// backoff returns how long to wait before retry number n (1 for
	// the first retry). Replaceable so tests need not wait.
	backoff func(n int) time.Duration
	// send performs one HTTP attempt and returns the status code.
	// Replaceable so tests can count attempts.
	send func(ep webhookEndpoint, data []byte) (int, error)
}

// newWebhookHub creates a hub from the webhook configuration map and
// starts its workers. Returns nil if no endpoints are configured.
func newWebhookHub(webhook map[string]config.WebhookConfig) *webhookHub {
	return newWebhookHubSized(webhook, webhookWorkers, webhookQueue)
}

// newWebhookHubSized is newWebhookHub with the worker count and queue
// depth chosen by the caller.
func newWebhookHubSized(webhook map[string]config.WebhookConfig, workers, depth int) *webhookHub {
	if len(webhook) == 0 {
		return nil
	}

	h := &webhookHub{
		queue:   make(chan delivery, depth),
		done:    make(chan struct{}),
		backoff: exponentialBackoff,
		send:    postWebhook,
	}

	names := make([]string, 0, len(webhook))
	for name := range webhook {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wh := webhook[name]
		ep := webhookEndpoint{name: name, url: wh.URL, key: wh.Key, attempts: webhookRetries + 1, timeout: webhookTimeout}
		if wh.Retries > 0 {
			ep.attempts = wh.Retries + 1
		}
		if wh.Timeout != "" {
			if d, err := time.ParseDuration(wh.Timeout); err == nil && d > 0 {
				ep.timeout = d
			} else {
				slog.Warn("webhook: invalid timeout, using default", "name", name, "timeout", wh.Timeout, "default", webhookTimeout)
			}
		}
		h.endpoints = append(h.endpoints, ep)
	}

	for range workers {
		h.wg.Add(1)
		go h.worker()
	}
	return h
}

// exponentialBackoff waits 1s, 2s, 4s, ... capped at 30s.
func exponentialBackoff(n int) time.Duration {
	d := time.Second << (n - 1)
	if d > 30*time.Second || d <= 0 {
		return 30 * time.Second
	}
	return d
}

// Broadcast queues an event for every endpoint. It never blocks: when
// the queue is full the event is dropped for that endpoint with a
// warning. Called synchronously from the event bus.
func (h *webhookHub) Broadcast(e events.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		slog.Warn("webhook: marshalling event", "err", err)
		return
	}

	for _, ep := range h.endpoints {
		select {
		case h.queue <- delivery{ep: ep, data: data}:
		default:
			slog.Warn("webhook: queue full, event dropped", "name", ep.name, "type", e.Type)
		}
	}
}

// Close stops the workers. The attempt in progress on each worker
// finishes (bounded by its timeout); queued events are dropped.
func (h *webhookHub) Close() {
	h.closeOnce.Do(func() { close(h.done) })
	h.wg.Wait()
}

func (h *webhookHub) worker() {
	defer h.wg.Done()
	for {
		select {
		case d := <-h.queue:
			h.deliver(d)
		case <-h.done:
			return
		}
	}
}

// deliver sends one delivery, retrying on a network error or a 5xx
// response until the endpoint's attempts are used up.
func (h *webhookHub) deliver(d delivery) {
	for attempt := 1; ; attempt++ {
		status, err := h.send(d.ep, d.data)
		switch {
		case err == nil && status < 400:
			return
		case err == nil && status < 500:
			slog.Warn("webhook: rejected", "name", d.ep.name, "url", d.ep.url, "status", status)
			return
		}
		if attempt >= d.ep.attempts {
			slog.Warn("webhook: delivery failed", "name", d.ep.name, "url", d.ep.url, "attempts", attempt, "status", status, "err", err)
			return
		}
		wait := h.backoff(attempt)
		slog.Debug("webhook: retrying", "name", d.ep.name, "attempt", attempt+1, "in", wait, "status", status, "err", err)
		select {
		case <-time.After(wait):
		case <-h.done:
			return
		}
	}
}

// postWebhook performs one HTTP POST of the payload to the endpoint.
func postWebhook(ep webhookEndpoint, data []byte) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ep.timeout)
	defer cancel()

	opt := options.New().
		AddHeader("Content-Type", "application/json").
		SetContext(ctx)
	if ep.key != "" {
		opt.AddHeader("Authorization", "Bearer "+ep.key)
	}
	resp, err := client.Post(ep.url, data, opt)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}
