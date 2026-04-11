// handler.go builds HTTP handlers that dispatch to sdk commands.
//
// Each handler extracts the path from the URL, reads metadata from
// headers, reads the body as stdin, and calls [sdk.Dispatch]. The
// response is written as JSON or plain text depending on the Output
// header.

package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jpl-au/llmd/internal/telemetry"
	"github.com/jpl-au/llmd/sdk"
)

// subcommandRoutes lists commands that use subcommand dispatching.
// For these, the URL path is split on "/" so each segment becomes a
// separate arg (e.g. /task/move/abc → ["move", "abc"]). All other
// commands receive the full path as a single arg.
var subcommandRoutes = map[string]bool{
	"task": true, "agent": true, "audit": true, "queue": true,
	"mirror": true, "tag": true, "link": true,
}

// handle returns an http.HandlerFunc that dispatches to the named
// command. The URL path after the command name becomes the command's
// positional arguments. Headers carry metadata. The request body,
// when present, is passed as stdin.
func (s *Server) handle(cmd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build args from the URL path after the command prefix.
		path := r.PathValue("path")
		args := buildArgs(cmd, r, path)

		// Read metadata from headers.
		author := r.Header.Get("Author")
		if author == "" {
			author = s.author
		}
		if msg := r.Header.Get("Message"); msg != "" {
			args = append(args, "--message", msg)
		}
		if src := r.Header.Get("Source"); src != "" {
			args = append(args, "--source", src)
		}

		// Read request body as stdin.
		var stdin []byte
		if r.Body != nil {
			defer r.Body.Close()
			b, err := io.ReadAll(r.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if len(b) > 0 {
				stdin = b
			}
		}

		result, err := sdk.Dispatch(r.Context(), cmd, args, author, stdin, "")
		telemetry.Emit(telemetry.Entry{
			Source:  "http",
			Command: cmd,
			Args:    args,
			Author:  author,
			Success: err == nil,
			Error:   telemetry.ErrStr(err),
		})
		if err != nil {
			writeDispatchError(w, err)
			return
		}

		writeResponse(w, r, result)
	}
}

// buildArgs constructs the argument slice from the URL path and query
// parameters. The "q" query parameter is treated as a positional
// argument (prepended before the path) for search commands. All other
// query parameters become flags (e.g. ?version=3 → --version 3).
//
// For subcommand dispatchers (task, agent, audit, queue), the path
// is split on "/" so each segment becomes a separate arg. For all
// other commands, the path is passed as a single arg.
func buildArgs(cmd string, r *http.Request, path string) []string {
	var args []string
	query := r.URL.Query()

	// The "q" param is a positional arg (search pattern), not a flag.
	if q := query.Get("q"); q != "" {
		args = append(args, q)
	}

	if path != "" {
		if subcommandRoutes[cmd] {
			args = append(args, strings.Split(path, "/")...)
		} else {
			args = append(args, path)
		}
	}

	for k, vals := range query {
		if k == "q" {
			continue
		}
		for _, v := range vals {
			args = append(args, "--"+k, v)
		}
	}
	return args
}

// writeResponse writes the sdk.Response to the HTTP response. When the
// Output header is "json", structured data is always returned. Otherwise
// text responses are returned as plain text and structured responses as
// JSON. Markdown responses are returned as raw markdown source over
// HTTP - the server does not render to terminal escape codes; that
// decision belongs to the consumer.
func writeResponse(w http.ResponseWriter, r *http.Request, resp sdk.Response) {
	wantJSON := strings.EqualFold(r.Header.Get("Output"), "json")

	switch v := resp.(type) {
	case sdk.Text:
		if wantJSON {
			writeJSON(w, http.StatusOK, v)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, string(v)); err != nil {
			slog.Warn("writing text response", "error", err)
		}

	case sdk.Markdown:
		// Markdown is returned raw over HTTP. Clients that want a
		// rendered view do their own rendering; clients that want
		// structured data can ask for JSON.
		if wantJSON || v.Text == "" {
			writeJSON(w, http.StatusOK, v.Data)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, v.Text); err != nil {
			slog.Warn("writing markdown response", "error", err)
		}

	case sdk.Result:
		// Result carries both text and structured data. Return JSON
		// when explicitly requested or when there is no text form.
		// Otherwise return the text representation.
		if wantJSON || v.Text == "" {
			writeJSON(w, http.StatusOK, v.Data)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, v.Text); err != nil {
			slog.Warn("writing text response", "error", err)
		}

	case sdk.Data:
		writeJSON(w, http.StatusOK, v.V)

	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeDispatchError maps sdk errors to HTTP status codes.
func writeDispatchError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, sdk.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, sdk.ErrMissingArg), errors.Is(err, sdk.ErrInvalidArg):
		code = http.StatusBadRequest
	case errors.Is(err, sdk.ErrExists):
		code = http.StatusConflict
	case errors.Is(err, sdk.ErrNoSpec):
		code = http.StatusUnprocessableEntity
	case errors.Is(err, sdk.ErrOrderViolation):
		code = http.StatusConflict
	case errors.Is(err, sdk.ErrCycle):
		code = http.StatusConflict
	}
	writeError(w, code, err)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	body := map[string]string{"error": err.Error()}
	if e := json.NewEncoder(w).Encode(body); e != nil {
		slog.Warn("writing error response", "error", e)
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("writing JSON response", "error", err)
	}
}
