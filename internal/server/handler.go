// handler.go builds HTTP handlers that dispatch to sdk commands.
//
// Each handler builds the command's arguments from the route's
// template and the URL, reads metadata from headers, reads the body as
// stdin, and dispatches through the runtime. The response is written
// as JSON or plain text depending on the Output header.

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

// handle returns an http.HandlerFunc that dispatches to the named
// command with the arguments described by tmpl (see sdk.Route.Args).
// Headers carry metadata. The request body, when present, is passed
// as stdin.
func (s *Server) handle(cmd string, tmpl []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		args := buildArgs(r, tmpl)

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

		result, err := s.rt.Dispatch(r.Context(), cmd, args, author, stdin, "")
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

// buildArgs constructs the argument slice for a request.
//
// The "q" query parameter comes first as a positional argument (the
// search pattern for grep and find). Then the route template is
// expanded: a "{name}" entry becomes the matching part of the URL and
// is dropped when empty; any other entry is passed as written. Every
// other query parameter becomes a flag: ?version=3 is --version 3,
// ?all=true is --all, and ?all=false is left out.
func buildArgs(r *http.Request, tmpl []string) []string {
	var args []string
	query := r.URL.Query()

	if q := query.Get("q"); q != "" {
		args = append(args, q)
	}

	for _, a := range tmpl {
		if strings.HasPrefix(a, "{") && strings.HasSuffix(a, "}") {
			if v := r.PathValue(a[1 : len(a)-1]); v != "" {
				args = append(args, v)
			}
			continue
		}
		args = append(args, a)
	}

	for k, vals := range query {
		if k == "q" {
			continue
		}
		for _, v := range vals {
			switch v {
			case "true":
				args = append(args, "--"+k)
			case "false":
			default:
				args = append(args, "--"+k, v)
			}
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
	case errors.Is(err, sdk.ErrAgentRunning), errors.Is(err, sdk.ErrAgentNotRunning), errors.Is(err, sdk.ErrNotReady):
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
