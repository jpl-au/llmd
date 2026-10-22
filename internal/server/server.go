// Package server provides an HTTP API for llmd.
//
// The server is a thin transport layer over [sdk.Runtime.Dispatch],
// following the same pattern as the MCP server. Each registered command becomes
// an HTTP route: reads are GET, mutations are POST. The request body
// carries document content (raw markdown), and headers carry metadata
// (Author, Message, Source). Responses are JSON by default, or raw
// content when appropriate.
//
// Commands are registered automatically by walking the runtime's
// command table, so extensions that register commands get HTTP routes
// for free.
//
// The /events endpoint streams real-time store events via SSE. Agents
// and external systems can subscribe to receive notifications when
// documents, audits, tasks, tags, or links change.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jpl-au/chain"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

// Server is an HTTP API server for llmd. It dispatches incoming
// requests to sdk commands through the runtime, using the same
// dispatch mechanism as the MCP server. The SSE hub streams events to
// connected clients.
type Server struct {
	mux    *chain.Mux
	rt     sdk.Runtime
	addr   string
	author string
	sse    *sseHub
}

// New creates a server from the given config and runtime. The server
// registers an SSE hub that streams store events to connected clients
// via GET /events, and a webhook hub that POSTs events to configured
// endpoints; both receive events only while the runtime has a store.
func New(cfg config.Config, rt sdk.Runtime) *Server {
	s := &Server{
		mux:    chain.New(),
		rt:     rt,
		addr:   cfg.Server.Addr,
		author: cfg.Author,
		sse:    newSSEHub(),
	}
	s.mux.Use(s.log)
	s.mux.HandleFunc("POST /hook", s.hookHandler())
	s.register()

	rt.Subscribe(s.sse.Broadcast)
	s.mux.Handle("GET /events", s.sse)

	if wh := newWebhookHub(cfg.Webhook); wh != nil {
		rt.Subscribe(wh.Broadcast)
		slog.Info("webhook enabled", "endpoints", len(wh.endpoints))
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

// register creates the HTTP routes for every command. A command that
// declares routes gets exactly those; any other command gets the two
// default routes from defaultRoutes.
func (s *Server) register() {
	for _, cmd := range s.rt.Commands() {
		if skip(cmd.Name) {
			continue
		}
		routes := cmd.Routes
		if len(routes) == 0 {
			routes = defaultRoutes(cmd)
		}
		for _, rt := range routes {
			s.mux.HandleFunc(rt.Method+" "+rt.Pattern, s.handle(cmd.Name, rt.Args))
		}
	}
}

// defaultRoutes gives a command without declared routes one route for
// the bare command and one with a trailing document path: GET for
// reads, POST for a command that needs an author.
func defaultRoutes(cmd *sdk.Command) []sdk.Route {
	method := "GET"
	if cmd.NeedsAuthor {
		method = "POST"
	}
	return []sdk.Route{
		{Method: method, Pattern: "/" + cmd.Name},
		{Method: method, Pattern: "/" + cmd.Name + "/{path...}", Args: []string{"{path}"}},
	}
}

// skip returns true for commands that should not be exposed over HTTP.
func skip(name string) bool {
	switch name {
	case "mcp", "serve", "init", "config", "version", "extensions":
		return true
	}
	return false
}
