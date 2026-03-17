// listener is a minimal HTTP server that captures POST requests and
// writes each body to stdout. Used for smoke-testing webhook delivery.
//
// Usage:
//
//	go run ./test/listener
//
// The server listens on :9999. POST requests are logged to stdout.
// DELETE /shutdown stops the server.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	srv := &http.Server{Addr: ":9999"}

	http.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		auth := r.Header.Get("Authorization")
		fmt.Fprintf(os.Stdout, "auth=%s body=%s\n", auth, body)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("DELETE /shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		go srv.Shutdown(context.Background())
	})

	log.Fatal(srv.ListenAndServe())
}
