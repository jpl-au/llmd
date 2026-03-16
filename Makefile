# Makefile — build targets for llmd.
#
# The standard build produces a binary with no diagnostic logging.
# The telemetry build includes compile-time telemetry that records
# every command dispatched through CLI, MCP, and HTTP to
# .llmd/telemetry.jsonl. This is useful for debugging integrations
# and understanding how agents interact with the store.
#
# Telemetry is controlled by a Go build tag, not a runtime flag.
# The standard binary has zero telemetry code compiled in.

MODULE  := github.com/jpl-au/llmd
TAG     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dirty)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null)
BUILT   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/app.Tag=$(TAG) \
	-X $(MODULE)/app.Commit=$(COMMIT) \
	-X $(MODULE)/app.Built=$(BUILT)

.PHONY: build build-telemetry test tidy clean

# Standard build — no telemetry.
build:
	go build -ldflags="$(LDFLAGS)" -o llmd .

# Telemetry build — records all commands to .llmd/telemetry.jsonl.
build-telemetry:
	go build -tags telemetry -ldflags="$(LDFLAGS)" -o llmd .

# Run all smoke tests (cli, http, telemetry).
test:
	bash test/run.sh

# Full tidy sequence — run before committing.
tidy:
	go fix ./...
	goimports -w .
	go fmt ./...
	go vet ./...
	go build ./...
	go test ./...

# Remove build artifacts.
clean:
	rm -f llmd test/llmd test/llmd-telem
