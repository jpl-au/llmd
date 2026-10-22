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
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jpl-au/chain"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

// DefaultAddr is the address the server listens on unless WithAddr
// says otherwise.
const DefaultAddr = "localhost:5563"

// Server is an HTTP API server for llmd. It dispatches incoming
// requests to sdk commands through the runtime, using the same
// dispatch mechanism as the MCP server. The SSE hub streams events to
// connected clients.
type Server struct {
	mux      *chain.Mux
	rt       sdk.Runtime
	addr     string
	author   string
	token    string
	webhooks map[string]config.WebhookConfig
	sse      *sseHub
	webhook  *webhookHub
}

// Option configures a Server.
type Option func(*Server)

// WithAddr sets the listen address. The default is DefaultAddr.
func WithAddr(addr string) Option {
	return func(s *Server) {
		if addr != "" {
			s.addr = addr
		}
	}
}

// WithAuthor sets the author used for requests that carry no Author
// header. The default is none, so such requests to commands that need
// an author are rejected.
func WithAuthor(author string) Option {
	return func(s *Server) { s.author = author }
}

// WithWebhooks sets the endpoints that receive every store event as a
// POST. The default is none.
func WithWebhooks(hooks map[string]config.WebhookConfig) Option {
	return func(s *Server) { s.webhooks = hooks }
}

// WithToken requires every request to carry "Authorization: Bearer
// <token>". The default is no token, which suits the localhost
// default address; set one before binding to any other address.
func WithToken(token string) Option {
	return func(s *Server) { s.token = token }
}

// New creates a server over the runtime. It registers a route for
// every command, an SSE hub that streams store events to clients on
// GET /events, and a webhook hub for any configured endpoints. Events
// flow only while the runtime has a store.
func New(rt sdk.Runtime, opts ...Option) *Server {
	s := &Server{
		mux:  chain.New(),
		rt:   rt,
		addr: DefaultAddr,
		sse:  newSSEHub(),
	}
	for _, o := range opts {
		o(s)
	}
	s.mux.Use(s.log)
	s.mux.Use(s.auth)
	s.register()

	rt.Subscribe(s.sse.Broadcast)
	s.mux.Handle("GET /events", s.sse)

	if s.webhook = newWebhookHub(s.webhooks); s.webhook != nil {
		rt.Subscribe(s.webhook.Broadcast)
		slog.Info("webhook enabled", "endpoints", len(s.webhook.endpoints))
	}

	return s
}

// errUnauthorized is returned when a token is required and missing or
// wrong.
var errUnauthorized = errors.New("unauthorised: a bearer token is required")

// auth is a middleware that enforces the configured bearer token. With
// no token configured it passes every request through.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeError(w, http.StatusUnauthorized, errUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
	if s.webhook != nil {
		defer s.webhook.Close()
	}

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
