# Integration Tests

Shell-based smoke tests that build the `llmd` binary and exercise it
end-to-end. Each suite runs in a temporary directory with a fresh store
and cleans up after itself.

## Usage

```bash
./run.sh              # run all suites
./run.sh cli          # run CLI tests only
./run.sh http         # run HTTP server tests only
./run.sh cli http     # run specific suites
```

## Suites

| File | Description |
|------|-------------|
| `smoke_cli.sh` | Core CLI commands: init, write, cat, ls, grep, sed, tag, mv, rm, restore, revert, diff, task, version |
| `smoke_http.sh` | HTTP API: GET/POST routes, content round-trips, JSON output, error codes |
| `smoke_telemetry.sh` | Telemetry and observability |
| `smoke_webhooks.sh` | Webhook delivery: event broadcast, auth headers, payload verification |

## Supporting tools

| Directory | Description |
|-----------|-------------|
| `listener/` | Minimal HTTP server for webhook smoke tests. Logs POST bodies to stdout, shuts down on `DELETE /shutdown`. |

## Adding a new suite

Create `smoke_<name>.sh` in this directory. The script is sourced by
`run.sh`, so it shares `$LLMD_BIN`, `log_pass`, and `log_fail`. Wrap
your tests in a function to keep variables local:

```bash
_smoke_name() {
    local work_dir llmd out
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"
    cd "$work_dir"

    # ... tests ...

    cd /
    rm -rf "$work_dir"
}

_smoke_name
```

Then run with `./run.sh name`.
