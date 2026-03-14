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
package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jpl-au/chain"
	"github.com/jpl-au/llmd/sdk"
)

// Server is an HTTP API server for llmd. It dispatches incoming
// requests to sdk commands, using the same dispatch mechanism as
// the MCP server.
type Server struct {
	mux    *chain.Mux
	addr   string
	author string
}

// New creates a server listening on addr. The author is the fallback
// identity used when requests omit the Author header.
func New(addr, author string) *Server {
	s := &Server{
		mux:    chain.New(),
		addr:   addr,
		author: author,
	}
	s.register()
	return s
}

// ListenAndServe starts the HTTP server. It blocks until the server
// is shut down or returns an error.
func (s *Server) ListenAndServe() error {
	slog.Info("starting HTTP server", "addr", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
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
