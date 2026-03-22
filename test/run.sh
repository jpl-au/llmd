#!/usr/bin/env bash
# run.sh — build llmd and run smoke tests.
#
# Usage:
#   ./run.sh              # run all smoke tests
#   ./run.sh cli          # run only CLI smoke tests
#   ./run.sh http         # run only HTTP smoke tests
#   ./run.sh cli http     # run specific suites

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LLMD_SRC="$SCRIPT_DIR/.."
LLMD_BIN="$SCRIPT_DIR/llmd"

# Colours for output.
RED='\033[0;31m'
GREEN='\033[0;32m'
BOLD='\033[1m'
RESET='\033[0m'

pass=0
fail=0

log_pass() { echo -e "  ${GREEN}✓${RESET} $1"; pass=$((pass + 1)); }
log_fail() { echo -e "  ${RED}✗${RESET} $1"; fail=$((fail + 1)); }

# Clean up built binary on exit regardless of outcome.
trap 'rm -f "$LLMD_BIN"' EXIT

# Build llmd from source.
echo -e "${BOLD}Building llmd...${RESET}"
if ! (cd "$LLMD_SRC" && go build -o "$LLMD_BIN" .); then
    echo -e "${RED}Build failed${RESET}"
    exit 1
fi
echo -e "  Built: $LLMD_BIN"
echo

# Determine which suites to run.
suites=("$@")
if [ ${#suites[@]} -eq 0 ]; then
    suites=(cli http queue deps telemetry webhooks)
fi

for suite in "${suites[@]}"; do
    script="$SCRIPT_DIR/smoke_${suite}.sh"
    if [ ! -f "$script" ]; then
        echo -e "${RED}Unknown suite: ${suite}${RESET} (no smoke_${suite}.sh found)"
        fail=$((fail + 1))
        continue
    fi

    echo -e "${BOLD}Running ${suite} smoke tests...${RESET}"
    # Each smoke script is sourced so it shares log_pass/log_fail
    # and the LLMD_BIN variable.
    source "$script"
    echo
done

# Summary.
total=$((pass + fail))
echo -e "${BOLD}Results: ${pass}/${total} passed${RESET}"
if [ "$fail" -gt 0 ]; then
    echo -e "${RED}${fail} test(s) failed${RESET}"
    exit 1
fi
echo -e "${GREEN}All tests passed${RESET}"
