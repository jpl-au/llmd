package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestAuth covers the bearer token middleware: with no token every
// request passes, with one the header must match exactly, and the
// refusal is a JSON error on every route including the event stream.
func TestAuth(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		header     string
		path       string
		wantStatus int
	}{
		{name: "no token configured", path: "/ls", wantStatus: http.StatusOK},
		{name: "no token configured ignores header", header: "Bearer anything", path: "/ls", wantStatus: http.StatusOK},
		{name: "matching token", token: "s3cret", header: "Bearer s3cret", path: "/ls", wantStatus: http.StatusOK},
		{name: "missing header", token: "s3cret", path: "/ls", wantStatus: http.StatusUnauthorized},
		{name: "wrong token", token: "s3cret", header: "Bearer other", path: "/ls", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", token: "s3cret", header: "Basic s3cret", path: "/ls", wantStatus: http.StatusUnauthorized},
		{name: "prefix only", token: "s3cret", header: "Bearer s3c", path: "/ls", wantStatus: http.StatusUnauthorized},
		{name: "events stream", token: "s3cret", path: "/events", wantStatus: http.StatusUnauthorized},
		{name: "unknown route", token: "s3cret", path: "/nope", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &stub{
				cmds: map[string]*sdk.Command{"ls": {Name: "ls"}},
				dispatch: func(context.Context, string, []string, string, []byte, string) (sdk.Response, error) {
					return sdk.Text("ok"), nil
				},
			}
			s := New(rt, WithAddr("localhost:0"), WithAuthor("test"), WithToken(tt.token))
			ts := httptest.NewServer(s.mux)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("GET %s status = %d, want %d", tt.path, resp.StatusCode, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusUnauthorized {
				return
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decoding error body: %v", err)
			}
			if body["error"] == "" {
				t.Errorf("error body = %v, want an error message", body)
			}
		})
	}
}
