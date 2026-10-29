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
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
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
		// Read metadata from headers.
		author := r.Header.Get("Author")
		if author == "" {
			author = s.author
		}
		var extra []string
		if msg := r.Header.Get("Message"); msg != "" {
			extra = append(extra, "--message", msg)
		}

		args, err := buildArgs(r, tmpl, extra...)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		// Read request body as stdin.
		var stdin []byte
		if r.Body != nil {
			defer r.Body.Close()
			b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxBody))
			if tooBig, ok := errors.AsType[*http.MaxBytesError](err); ok {
				writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("the request body is larger than %d bytes.", tooBig.Limit))
				return
			}
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
// is dropped when empty, a "{?name}" entry becomes that query
// parameter, and any other entry is passed as written. Every other
// query parameter becomes a flag, in name order: ?version=3 is
// --version 3, ?all=true is --all, and ?all=false is left out. extra
// are flags from headers (--message).
//
// A value from a URL path that starts with "-" is refused: the command
// would read it as a flag, so a path segment could switch on an option
// the route never offered. Free text from a "{?name}" parameter may
// start with "-" (a markdown list item does), so on such routes the
// flags come first and "--" ends them before the values. Unknown query
// names are refused by the command's own flag parsing.
func buildArgs(r *http.Request, tmpl []string, extra ...string) ([]string, error) {
	query := r.URL.Query()
	var lead, values []string // subcommand literals, then positional values
	freeText := map[string]bool{}

	if q := query.Get("q"); q != "" {
		if strings.HasPrefix(q, "-") {
			return nil, sdk.Errorf(sdk.ErrInvalidArg, "the query %s cannot start with -.", q)
		}
		values = append(values, q)
	}

	for _, a := range tmpl {
		switch {
		case strings.HasPrefix(a, "{?") && strings.HasSuffix(a, "}"):
			name := a[2 : len(a)-1]
			freeText[name] = true
			if v := query.Get(name); v != "" {
				values = append(values, v)
			}
		case strings.HasPrefix(a, "{") && strings.HasSuffix(a, "}"):
			name := a[1 : len(a)-1]
			if v := r.PathValue(name); v != "" {
				if strings.HasPrefix(v, "-") {
					return nil, sdk.Errorf(sdk.ErrInvalidArg, "the %s %s cannot start with -.", name, v)
				}
				values = append(values, v)
			}
		case len(values) == 0:
			lead = append(lead, a)
		default:
			values = append(values, a)
		}
	}

	var flags []string
	for _, k := range slices.Sorted(maps.Keys(query)) {
		if k == "q" || freeText[k] {
			continue
		}
		for _, v := range query[k] {
			switch v {
			case "true":
				flags = append(flags, "--"+k)
			case "false":
			default:
				flags = append(flags, "--"+k, v)
			}
		}
	}
	flags = append(flags, extra...)

	args := slices.Clone(lead)
	if len(freeText) > 0 {
		args = append(args, flags...)
		args = append(args, "--")
		return append(args, values...), nil
	}
	args = append(args, values...)
	return append(args, flags...), nil
}

// writeResponse writes the sdk.Response to the HTTP response. When the
// Output header is "json", structured data is always returned. Otherwise
// text responses are returned as plain text and structured responses as
// JSON. Markdown responses are returned as raw markdown source over
// HTTP - the server does not render to terminal escape codes; that
// decision belongs to the consumer.
func writeResponse(w http.ResponseWriter, r *http.Request, resp sdk.Response) {
	wantJSON := strings.EqualFold(r.Header.Get("Output"), "json")

	// The text form, when there is one and JSON was not asked for.
	// Markdown is returned raw: clients that want a rendered view do
	// their own rendering.
	var text, contentType string
	switch v := resp.(type) {
	case sdk.Text:
		text, contentType = string(v), "text/plain; charset=utf-8"
	case sdk.Markdown:
		text, contentType = v.Text, "text/markdown; charset=utf-8"
	case sdk.Result:
		text, contentType = v.Text, "text/plain; charset=utf-8"
	case sdk.Data:
		writeJSON(w, http.StatusOK, sdk.JSON(v))
		return
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// JSON when asked for, or when a structured response has no text.
	if wantJSON || (text == "" && hasData(resp)) {
		writeJSON(w, http.StatusOK, sdk.JSON(resp))
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, text); err != nil {
		slog.Warn("writing text response", "error", err)
	}
}

// hasData reports whether a response carries structured data.
func hasData(resp sdk.Response) bool {
	switch v := resp.(type) {
	case sdk.Markdown:
		return v.Data != nil
	case sdk.Result:
		return v.Data != nil
	}
	return false
}

// writeDispatchError maps sdk errors to HTTP status codes.
func writeDispatchError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	for _, m := range statusOf {
		if errors.Is(err, m.kind) {
			code = m.code
			break
		}
	}
	writeError(w, code, err)
}

// statusOf maps each SDK error kind to its HTTP status. Every kind in
// sdk has an entry (TestStatusOfEveryKind): one left out reached
// clients as a 500, as if the server had failed.
var statusOf = []struct {
	kind error
	code int
}{
	{sdk.ErrNotFound, http.StatusNotFound},
	{sdk.ErrUnknownCmd, http.StatusNotFound},
	{sdk.ErrMissingArg, http.StatusBadRequest},
	{sdk.ErrInvalidArg, http.StatusBadRequest},
	{sdk.ErrExists, http.StatusConflict},
	{sdk.ErrOrderViolation, http.StatusConflict},
	{sdk.ErrCycle, http.StatusConflict},
	{sdk.ErrAgentRunning, http.StatusConflict},
	{sdk.ErrAgentNotRunning, http.StatusConflict},
	{sdk.ErrNotReady, http.StatusConflict},
	{sdk.ErrNoneReady, http.StatusConflict},
	{sdk.ErrUnavailable, http.StatusConflict},
	{sdk.ErrNoSpec, http.StatusUnprocessableEntity},
	{sdk.ErrNoMatch, http.StatusUnprocessableEntity},
	{sdk.ErrNotUnique, http.StatusUnprocessableEntity},
	{sdk.ErrNoOp, http.StatusUnprocessableEntity},
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
