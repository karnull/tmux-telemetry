#!/usr/bin/env bash

# A gap from a restart goes down as AFK, a long one ends the day, and the watcher
# ends the day when tmux stops

for tool in tmux setsid; do
    command -v "$tool" >/dev/null || { echo "watch: skipped, needs $tool"; exit 0; }
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH=$(mktemp -d)
trap 'tmux kill-server 2>/dev/null; rm -rf "$SCRATCH"' EXIT

unset TMUX
export TMUX_TMPDIR="$SCRATCH" TELEMETRY_DIR="$SCRATCH/logs" TELEMETRY_TICK=1 TZ=UTC
failed=0

events() { cut -d, -f1,2 "$TELEMETRY_DIR"/*.csv | grep -v '^timestamp'; }

check() {
    if [ "$2" == "$3" ]; then
        return
    fi
    echo "watch: $1"
    diff -u <(echo "$3") <(echo "$2")
    failed=1
}

# 2001-09-09T01:46:40Z, then five minutes and three hours on
"$ROOT/scripts/telemetry.sh" resume 1000000000 1000000300
"$ROOT/scripts/telemetry.sh" resume 1000000000 1000010800
check "short and long gaps" "$(events)" "$(printf '%s\n' \
    '2001-09-09T01:46:40+00:00,afk' \
    '2001-09-09T01:46:40+00:00,away')"
rm -rf "$TELEMETRY_DIR"

# a tmux started two minutes after the last one stopped
mkdir -p "$TELEMETRY_DIR"
gone=$(( $(date +%s) - 120 ))
echo "$gone" > "$TELEMETRY_DIR/.alive"
tmux -f /dev/null new-session -d -s Work
tmux run-shell "$ROOT/telemetry.tmux"
sleep 2
check "a restart is AFK" "$(events | grep afk)" "$(TZ=UTC printf '%(%Y-%m-%dT%H:%M:%S)T+00:00,afk' "$gone")"

[ -s "$TELEMETRY_DIR/.watcher" ] || { echo "watch: no watcher running"; failed=1; }
watcher=$(< "$TELEMETRY_DIR/.watcher")

# a second start, as from reloading the config, leaves the one watcher
tmux run-shell "$ROOT/telemetry.tmux"
sleep 1
check "one watcher" "$(< "$TELEMETRY_DIR/.watcher")" "$watcher"

# a watcher stopped for a while, as the machine is when it sleeps, notices on waking
kill -STOP "$watcher"
sleep 4
kill -CONT "$watcher"
sleep 2
check "a sleep is AFK" "$(events | grep -c afk)" "2"

tmux kill-server
sleep 3
check "tmux stopping ends the day" "$(events | tail -n 1 | cut -d, -f2)" "away"
[ -e "$TELEMETRY_DIR/.watcher" ] && { echo "watch: the watcher did not stop"; failed=1; }

(( failed )) && exit 1
echo "watch: ok"
