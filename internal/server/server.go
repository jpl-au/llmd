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
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jpl-au/chain"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

// DefaultAddr is the address the server listens on unless WithAddr
// says otherwise.
const DefaultAddr = config.DefaultAddr

// DefaultWatchInterval is how often the server looks for changes
// other processes made to the store (see sdk.Runtime.Watch).
const DefaultWatchInterval = time.Second

// Defaults for the HTTP server's limits. ReadHeaderTimeout and
// ReadTimeout stop a client holding a connection open by sending a
// request slowly; IdleTimeout closes keep-alive connections nobody is
// using. There is no write timeout: /events streams for as long as the
// client listens. DefaultMaxBody is the default content limit
// (limits.content_size) plus room for the request around it.
const (
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = time.Minute
	DefaultIdleTimeout       = 2 * time.Minute
	DefaultShutdownTimeout   = 5 * time.Second
	DefaultMaxBody           = 10<<20 + 64<<10
)

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
	workers  int // webhook workers; zero means the default
	depth    int // webhook queue depth; zero means the default
	sse      *sseHub
	webhook  *webhookHub

	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	idleTimeout       time.Duration
	shutdownTimeout   time.Duration
	watchInterval     time.Duration
	maxBody           int64
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

// WithWebhookQueue sets how many workers deliver webhooks and how many
// events may wait for them. Zero or less keeps the defaults of four
// workers and a queue of 256.
func WithWebhookQueue(workers, depth int) Option {
	return func(s *Server) { s.workers, s.depth = workers, depth }
}

// WithToken requires every request to carry "Authorization: Bearer
// <token>". The default is no token, which suits the localhost
// default address; set one before binding to any other address.
func WithToken(token string) Option {
	return func(s *Server) { s.token = token }
}

// WithTimeouts sets how long a client may take to send request headers,
// to send the whole request, and how long an idle keep-alive connection
// stays open. Zero or less keeps a default.
func WithTimeouts(readHeader, read, idle time.Duration) Option {
	return func(s *Server) {
		s.readHeaderTimeout = positive(readHeader, s.readHeaderTimeout)
		s.readTimeout = positive(read, s.readTimeout)
		s.idleTimeout = positive(idle, s.idleTimeout)
	}
}

// WithShutdownTimeout sets how long requests in flight may take to
// finish once the server is asked to stop. Zero or less keeps the
// default.
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.shutdownTimeout = positive(d, s.shutdownTimeout) }
}

// WithWatchInterval sets how often the server looks for changes other
// processes made to the store. Zero or less keeps the default.
func WithWatchInterval(d time.Duration) Option {
	return func(s *Server) { s.watchInterval = positive(d, s.watchInterval) }
}

// WithMaxBody sets the largest request body, in bytes, the server
// reads; a bigger one is refused with 413. Zero or less keeps the
// default.
func WithMaxBody(n int64) Option {
	return func(s *Server) {
		if n > 0 {
			s.maxBody = n
		}
	}
}

// positive returns d when it is above zero and fallback otherwise.
func positive(d, fallback time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return fallback
}

// New creates a server over the runtime. It registers a route for
// every command, an SSE hub that streams store events to clients on
// GET /events, and a webhook hub for any configured endpoints. Events
// flow only while the runtime has a store.
func New(rt sdk.Runtime, opts ...Option) *Server {
	s := &Server{
		mux:               chain.New(),
		rt:                rt,
		addr:              DefaultAddr,
		sse:               newSSEHub(),
		readHeaderTimeout: DefaultReadHeaderTimeout,
		readTimeout:       DefaultReadTimeout,
		idleTimeout:       DefaultIdleTimeout,
		shutdownTimeout:   DefaultShutdownTimeout,
		watchInterval:     DefaultWatchInterval,
		maxBody:           DefaultMaxBody,
	}
	for _, o := range opts {
		o(s)
	}
	s.mux.Use(s.log)
	s.mux.Use(s.local)
	s.mux.Use(s.auth)
	s.register()

	rt.Subscribe(s.sse.Broadcast)
	s.mux.Handle("GET /events", s.sse)

	if s.webhook = newWebhookHubSized(s.webhooks, s.workers, s.depth); s.webhook != nil {
		rt.Subscribe(s.webhook.Broadcast)
		slog.Info("webhook enabled", "endpoints", len(s.webhook.endpoints))
	}

	return s
}

// errUnauthorized is returned when a token is required and missing or
// wrong.
var errUnauthorized = errors.New("a bearer token is required. Send Authorization: Bearer <server.token>.")

// auth is a middleware that enforces the configured bearer token. With
// no token configured it passes every request through.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeError(w, http.StatusUnauthorized, errUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// local is a middleware that, when no token is set, answers only
// requests addressed to this machine. Without it a web page could reach
// the server through DNS rebinding: its own domain resolves to
// 127.0.0.1, so the browser treats the server as the page's origin and
// lets it send any request and read the reply. Such a request carries
// the page's domain in Host (and Origin), which is refused here. With a
// token the token is the check, and the server may be addressed by any
// name.
func (s *Server) local(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			if !s.allowedHost(r.Host) {
				writeError(w, http.StatusForbidden, fmt.Errorf("requests to %s are refused. Without server.token, llmd answers only requests to localhost.", r.Host))
				return
			}
			if o := r.Header.Get("Origin"); o != "" {
				u, err := url.Parse(o)
				if err != nil || !s.allowedHost(u.Host) {
					writeError(w, http.StatusForbidden, fmt.Errorf("requests from %s are refused. Without server.token, llmd answers only pages served from localhost.", o))
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost reports whether a Host or Origin host, with or without a
// port, names this machine: a loopback address, localhost, or the host
// the server was told to listen on.
func (s *Server) allowedHost(hostport string) bool {
	host := hostOnly(hostport)
	if loopback(host) {
		return true
	}
	listen := hostOnly(s.addr)
	return listen != "" && strings.EqualFold(host, listen)
}

// loopback reports whether host is localhost or a loopback address.
func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// hostOnly strips the port and IPv6 brackets from host:port.
func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

// ListenAndServe starts the HTTP server. It blocks until the context
// is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	// Without a token the Host check is the only protection, and it
	// only holds on loopback: anyone who can reach another address can
	// send any Host they like.
	if s.token == "" && !loopback(hostOnly(s.addr)) {
		return fmt.Errorf("serve: %s is not a loopback address, so server.token must be set. Run llmd config server.token <secret>.", s.addr)
	}

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: s.readHeaderTimeout,
		ReadTimeout:       s.readTimeout,
		IdleTimeout:       s.idleTimeout,
	}
	// Shutdown waits for every handler to return, and an /events
	// stream only returns when its client leaves. Ending the streams
	// when shutdown begins lets it finish.
	srv.RegisterOnShutdown(s.sse.Close)

	// The watcher stops with the server, whichever way it stops.
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	slog.Info("serving", "addr", s.addr)
	if s.webhook != nil {
		defer s.webhook.Close()
	}
	// Changes made by other processes (the command line, MCP sessions)
	// reach SSE and webhook clients as store.changed events.
	go func() {
		if err := s.rt.Watch(ctx, s.watchInterval); err != nil {
			slog.Warn("watching the store for outside changes", "err", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

// Handler returns the server's HTTP handler: the routes, with the
// router's own replies for an unknown path (404) or a wrong method
// (405) given as JSON like every other error, rather than net/http's
// plain text.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mux.ServeHTTP(&jsonRouterErrors{ResponseWriter: w, r: r}, r)
	})
}

// jsonRouterErrors rewrites the plain-text 404 and 405 replies
// net/http's router writes itself. Replies from llmd's handlers are
// JSON already and pass through untouched.
type jsonRouterErrors struct {
	http.ResponseWriter
	r        *http.Request
	replaced bool
}

func (j *jsonRouterErrors) WriteHeader(code int) {
	plain := strings.HasPrefix(j.Header().Get("Content-Type"), "text/plain")
	if !j.replaced && plain && (code == http.StatusNotFound || code == http.StatusMethodNotAllowed) {
		j.replaced = true
		j.Header().Del("X-Content-Type-Options")
		msg := fmt.Errorf("there is no route %s. Run llmd guide serve to list them.", j.r.URL.Path)
		if code == http.StatusMethodNotAllowed {
			msg = fmt.Errorf("%s does not accept %s. It accepts %s.", j.r.URL.Path, j.r.Method, j.Header().Get("Allow"))
		}
		writeError(j.ResponseWriter, code, msg)
		return
	}
	j.ResponseWriter.WriteHeader(code)
}

func (j *jsonRouterErrors) Write(b []byte) (int, error) {
	if j.replaced {
		return len(b), nil // the router's text, already replaced
	}
	return j.ResponseWriter.Write(b)
}

// Flush passes through so /events can stream.
func (j *jsonRouterErrors) Flush() {
	if f, ok := j.ResponseWriter.(http.Flusher); ok {
		f.Flush()
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
