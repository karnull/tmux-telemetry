#!/usr/bin/env bash

# The report for the fixtures matches the format and totals from the original request

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TELEMETRY_DIR=$(mktemp -d)
trap 'rm -rf "$TELEMETRY_DIR"' EXIT
cp "$ROOT"/tests/fixtures/*.tsv "$TELEMETRY_DIR"

# a private socket, so options set in a running tmux don't change the result
unset TMUX
export TELEMETRY_DIR TMUX_TMPDIR="$TELEMETRY_DIR"

if diff -u "$ROOT/tests/fixtures/expected.txt" <("$ROOT/scripts/telemetry.sh" all); then
    echo "report: ok"
else
    echo "report: failed"
    exit 1
fi
