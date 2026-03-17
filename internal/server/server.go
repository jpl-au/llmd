// Package server provides an HTTP API for llmd.
//
// The server is a thin transport layer over [sdk.Dispatch], following
// the same pattern as the MCP server. Each registered command becomes
// an HTTP route: reads are GET, mutations are POST. The request body
// carries document content (raw markdown), and headers carry metadata
// (Author, Message, Source). Responses are JSON by default, or raw
// content when appropriate.
//
// Commands are registered automatically by walking [sdk.AllCommands],
// so plugins that register commands get HTTP routes for free.
//
// The /events endpoint streams real-time store events via SSE. Agents
// and external systems can subscribe to receive notifications when
// documents, audits, tasks, tags, or links change.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jpl-au/chain"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// Server is an HTTP API server for llmd. It dispatches incoming
// requests to sdk commands, using the same dispatch mechanism as
// the MCP server. The SSE hub streams events to connected clients.
type Server struct {
	mux    *chain.Mux
	addr   string
	author string
	sse    *sseHub
}

// New creates a server from the given config. If subscribe is
// non-nil, the server registers an SSE hub that streams store
// events to connected clients via GET /events, and a webhook hub
// that POSTs events to configured endpoints.
func New(cfg config.Config, subscribe func(func(events.Event))) *Server {
	s := &Server{
		mux:    chain.New(),
		addr:   cfg.Server.Addr,
		author: cfg.Author,
		sse:    newSSEHub(),
	}
	s.mux.Use(s.log)
	s.register()

	if subscribe != nil {
		subscribe(s.sse.Broadcast)
		s.mux.Handle("GET /events", s.sse)

		if wh := newWebhookHub(cfg.Webhook); wh != nil {
			subscribe(wh.Broadcast)
			slog.Info("webhook enabled", "endpoints", len(wh.endpoints))
		}
	}

	return s
}

// ListenAndServe starts the HTTP server. It blocks until the context
// is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	slog.Info("serving", "addr", s.addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

// log is a middleware that records each request with slog.
func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		rw := w.(chain.ResponseWriter)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.Status(),
			"size", rw.Size(),
			"duration", time.Since(start),
		)
	})
}

// register walks all registered commands and creates an HTTP route
// for each one. Read commands (NeedsAuthor == false) are registered
// as GET, mutation commands as POST.
func (s *Server) register() {
	for _, cmd := range sdk.AllCommands() {
		if skip(cmd.Name) {
			continue
		}

		method := "GET"
		if cmd.NeedsAuthor {
			method = "POST"
		}

		pattern := fmt.Sprintf("%s /%s/{path...}", method, cmd.Name)
		s.mux.HandleFunc(pattern, s.handle(cmd.Name))

		// Also register the bare path without a trailing wildcard so
		// commands like /ls or /grep work without a path argument.
		bare := fmt.Sprintf("%s /%s", method, cmd.Name)
		s.mux.HandleFunc(bare, s.handle(cmd.Name))
	}
}

// skip returns true for commands that should not be exposed over HTTP.
func skip(name string) bool {
	switch name {
	case "mcp", "serve", "init", "config", "version", "plugins", "guide", "llm":
		return true
	}
	return false
}
