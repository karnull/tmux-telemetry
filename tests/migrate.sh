#!/usr/bin/env bash

# Daily .tsv records from older versions become one .csv a month, quoted where needed

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TELEMETRY_DIR=$(mktemp -d)
trap 'rm -rf "$TELEMETRY_DIR"' EXIT
cp "$ROOT/tests/fixtures/2026-11-10.tsv" "$TELEMETRY_DIR"
export TELEMETRY_DIR TZ=AEST-10AEDT,M10.1.0,M4.1.0/3   # POSIX rules, so no tzdata is needed

"$ROOT/scripts/telemetry.sh" migrate > /dev/null

if diff -u "$ROOT/tests/fixtures/migrated.csv" "$TELEMETRY_DIR/2026-11.csv" &&
    [ -e "$TELEMETRY_DIR/2026-11-10.tsv.migrated" ]; then
    echo "migrate: ok"
else
    echo "migrate: failed"
    exit 1
fi
