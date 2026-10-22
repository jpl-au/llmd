package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestWriteResponse covers how each sdk.Response type is written over
// HTTP: Text is plain text, Markdown is the raw source as
// text/markdown, Result is its text form, and Data is always JSON.
// The Output: json header switches Text, Markdown and Result to the
// JSON encoding of their data, as does a Markdown or Result with no
// text form. A nil response is 204 with no body. Each case checks the
// status, the Content-Type with its charset and the body, decoding
// JSON bodies to the expected shape.
func TestWriteResponse(t *testing.T) {
	const (
		textType     = "text/plain; charset=utf-8"
		markdownType = "text/markdown; charset=utf-8"
		jsonType     = "application/json; charset=utf-8"
	)
	data := map[string]any{"path": "docs/a", "version": 2}
	wantData := map[string]any{"path": "docs/a", "version": float64(2)}

	tests := []struct {
		name       string
		resp       sdk.Response
		output     string // Output header
		wantStatus int
		wantType   string
		wantBody   string // exact body for a text response
		wantJSON   any    // decoded body for a JSON response
	}{
		{name: "text", resp: sdk.Text("hello\n"), wantStatus: http.StatusOK, wantType: textType, wantBody: "hello\n"},
		{name: "text as json", resp: sdk.Text("hello"), output: "json", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: "hello"},
		{name: "text with another output value", resp: sdk.Text("hello"), output: "text", wantStatus: http.StatusOK, wantType: textType, wantBody: "hello"},
		{name: "output header is case-insensitive", resp: sdk.Text("hello"), output: "JSON", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: "hello"},
		{name: "markdown", resp: sdk.Markdown{Text: "# Hi\n\nBody.\n", Data: data}, wantStatus: http.StatusOK, wantType: markdownType, wantBody: "# Hi\n\nBody.\n"},
		{name: "markdown as json", resp: sdk.Markdown{Text: "# Hi\n", Data: data}, output: "json", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "markdown without text", resp: sdk.Markdown{Data: data}, wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "markdown without data as json", resp: sdk.Markdown{Text: "# Hi\n"}, output: "json", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: nil},
		{name: "result", resp: sdk.Result{Text: "docs/a  v2\n", Data: data}, wantStatus: http.StatusOK, wantType: textType, wantBody: "docs/a  v2\n"},
		{name: "result as json", resp: sdk.Result{Text: "docs/a  v2\n", Data: data}, output: "json", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "result without text", resp: sdk.Result{Data: data}, wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "data", resp: sdk.Data{V: data}, wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "data as json", resp: sdk.Data{V: data}, output: "json", wantStatus: http.StatusOK, wantType: jsonType, wantJSON: wantData},
		{name: "data list", resp: sdk.Data{V: []string{"a", "b"}}, wantStatus: http.StatusOK, wantType: jsonType, wantJSON: []any{"a", "b"}},
		{name: "nil response", resp: nil, wantStatus: http.StatusNoContent},
		{name: "nil response as json", resp: nil, output: "json", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := stubRuntime()
			rt.dispatch = func(_ context.Context, _ string, _ []string, _ string, _ []byte, _ string) (sdk.Response, error) {
				return tt.resp, nil
			}
			ts := newTestServer(t, rt)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/ls", nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			if tt.output != "" {
				req.Header.Set("Output", tt.output)
			}
			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("GET /ls: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("writeResponse(%#v, Output=%q) status = %d, want %d", tt.resp, tt.output, resp.StatusCode, tt.wantStatus)
			}
			if ct := resp.Header.Get("Content-Type"); ct != tt.wantType {
				t.Errorf("writeResponse(%#v, Output=%q) Content-Type = %q, want %q", tt.resp, tt.output, ct, tt.wantType)
			}
			if !strings.HasPrefix(tt.wantType, "application/json") {
				if string(body) != tt.wantBody {
					t.Errorf("writeResponse(%#v, Output=%q) body = %q, want %q", tt.resp, tt.output, body, tt.wantBody)
				}
				return
			}
			var got any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("writeResponse(%#v, Output=%q) body %q is not JSON: %v", tt.resp, tt.output, body, err)
			}
			if !reflect.DeepEqual(got, tt.wantJSON) {
				t.Errorf("writeResponse(%#v, Output=%q) decoded = %#v, want %#v", tt.resp, tt.output, got, tt.wantJSON)
			}
		})
	}
}

// TestHandleMetadata covers what reaches the dispatcher besides the
// URL: the Author header, falling back to the server's WithAuthor
// default when absent; the Message and Source headers as --message
// and --source arguments after the positional ones; and the request
// body as stdin, with an empty body passed as nil rather than an
// empty slice.
func TestHandleMetadata(t *testing.T) {
	tests := []struct {
		name          string
		defaultAuthor string // WithAuthor
		headers       map[string]string
		body          string
		wantAuthor    string
		wantArgs      []string
		wantStdin     []byte
	}{
		{
			name:       "author header",
			headers:    map[string]string{"Author": "claude"},
			wantAuthor: "claude",
			wantArgs:   []string{"docs/a"},
		},
		{
			name:          "author falls back to server default",
			defaultAuthor: "serve",
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a"},
		},
		{
			name:          "author header overrides server default",
			defaultAuthor: "serve",
			headers:       map[string]string{"Author": "gemini"},
			wantAuthor:    "gemini",
			wantArgs:      []string{"docs/a"},
		},
		{
			name:       "no author anywhere",
			wantAuthor: "",
			wantArgs:   []string{"docs/a"},
		},
		{
			name:          "message header",
			defaultAuthor: "serve",
			headers:       map[string]string{"Message": "fix typo"},
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a", "--message", "fix typo"},
		},
		{
			name:          "source header",
			defaultAuthor: "serve",
			headers:       map[string]string{"Source": "ci"},
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a", "--source", "ci"},
		},
		{
			name:          "message then source",
			defaultAuthor: "serve",
			headers:       map[string]string{"Source": "ci", "Message": "fix typo"},
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a", "--message", "fix typo", "--source", "ci"},
		},
		{
			name:          "body is stdin",
			defaultAuthor: "serve",
			body:          "# Hello\n\nBody.\n",
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a"},
			wantStdin:     []byte("# Hello\n\nBody.\n"),
		},
		{
			name:          "empty body is nil stdin",
			defaultAuthor: "serve",
			body:          "",
			wantAuthor:    "serve",
			wantArgs:      []string{"docs/a"},
			wantStdin:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := stubRuntime()
			rt.cmds = map[string]*sdk.Command{"edit": {Name: "edit", NeedsAuthor: true}}
			var mu sync.Mutex
			var gotAuthor string
			var gotArgs []string
			var gotStdin []byte
			rt.dispatch = func(_ context.Context, _ string, args []string, author string, stdin []byte, _ string) (sdk.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				gotAuthor, gotArgs, gotStdin = author, args, stdin
				return sdk.Text("ok"), nil
			}
			s := New(rt, WithAddr("localhost:0"), WithAuthor(tt.defaultAuthor))
			ts := httptest.NewServer(s.mux)
			t.Cleanup(ts.Close)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/edit/docs/a", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("POST /edit/docs/a: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("POST /edit/docs/a status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			mu.Lock()
			defer mu.Unlock()
			if gotAuthor != tt.wantAuthor {
				t.Errorf("handle(headers=%v) author = %q, want %q", tt.headers, gotAuthor, tt.wantAuthor)
			}
			if !slices.Equal(gotArgs, tt.wantArgs) {
				t.Errorf("handle(headers=%v) args = %q, want %q", tt.headers, gotArgs, tt.wantArgs)
			}
			if (gotStdin == nil) != (tt.wantStdin == nil) || !bytes.Equal(gotStdin, tt.wantStdin) {
				t.Errorf("handle(body=%q) stdin = %#v, want %#v", tt.body, gotStdin, tt.wantStdin)
			}
		})
	}
}
