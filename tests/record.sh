#!/usr/bin/env bash

# Drives a private tmux server with a real client attached, and checks what gets logged

command -v python3 >/dev/null || { echo "record: skipped, needs python3 for a pty"; exit 0; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH=$(mktemp -d)
trap 'tmux kill-server 2>/dev/null; wait; rm -rf "$SCRATCH"' EXIT

unset TMUX
export TMUX_TMPDIR="$SCRATCH" TELEMETRY_DIR="$SCRATCH/logs" TERM=xterm-256color
export ZDOTDIR="$SCRATCH/zsh" SHELL=zsh
mkdir -p "$ZDOTDIR" "$SCRATCH/repo" "$SCRATCH/plain" "$SCRATCH/Widget"
echo 'source "$TELEMETRY_HOME/telemetry.zsh"' > "$ZDOTDIR/.zshrc"
git -C "$SCRATCH/repo" init -q -b feat/test
git -C "$SCRATCH/Widget" init -q -b main

# the plugin loads before any shell starts, as it does from a tmux config
tmux -f /dev/null new-session -d -s setup
tmux set -g @telemetry-ignore popup
"$ROOT/telemetry.tmux"
tmux new-session -d -s Work -c "$SCRATCH/repo" zsh
tmux new-session -d -s Other -c "$SCRATCH/plain" zsh
tmux new-session -d -s popup zsh

attach() {
    python3 - "$1" <<'PY' &
import os, pty, sys, time
pid, fd = pty.fork()
if pid == 0:
    os.execvp("tmux", ["tmux", "attach", "-t", sys.argv[1]])
end = time.time() + 12
while time.time() < end:
    try:
        os.read(fd, 65536)
    except OSError:
        break
PY
}

step() { tmux "$@"; sleep 1; }

tmux kill-session -t setup
attach Work; sleep 2
step send -t Other 'true' Enter
step switch-client -c "$(tmux list-clients -F '#{client_name}' | head -n 1)" -t Other
step send -t Other 'cd /' Enter
step send -t Other "cd $SCRATCH/Widget" Enter
attach popup; sleep 2
step send -t popup 'true' Enter
step detach-client -s popup
step switch-client -t Work
step detach-client -s Work

actual=$(cut -d, -f 2- "$TELEMETRY_DIR"/*.csv | uniq)
expected=$(printf '%s\n' \
    'event,session,context' \
    'focus,Work,feat/test' \
    "focus,Other,$SCRATCH/plain" \
    'focus,Other,/' \
    'focus,Other,Widget' \
    'focus,Work,feat/test' \
    'away,,')

if [ "$actual" == "$expected" ]; then
    echo "record: ok"
else
    echo "record: failed"
    diff -u <(echo "$expected") <(echo "$actual")
    exit 1
fi
