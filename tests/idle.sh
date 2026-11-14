#!/usr/bin/env bash

# A quiet spell asks whether anyone is there; away leaves it UNATTENDED until
# whoever is back says where they were

for tool in python3 tmux setsid; do
    command -v "$tool" >/dev/null || { echo "idle: skipped, needs $tool"; exit 0; }
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH=$(mktemp -d)
trap 'tmux kill-server 2>/dev/null; wait; rm -rf "$SCRATCH"' EXIT

unset TMUX
export TMUX_TMPDIR="$SCRATCH" TELEMETRY_DIR="$SCRATCH/logs" TERM=xterm-256color TZ=UTC
failed=0

rows() { grep -hv '^timestamp' "$TELEMETRY_DIR"/*.csv; }

check() {
    if [ "$2" == "$3" ]; then
        return
    fi
    echo "idle: $1"
    diff -u <(echo "$3") <(echo "$2")
    failed=1
}

# each answer, a second after the spell began at 2001-09-09T01:46:40Z
for where in "" a lunch m e bench "fixing the printer"; do
    "$ROOT/scripts/telemetry.sh" answer 1000000000 Minecraft "$where"
done
"$ROOT/scripts/telemetry.sh" answer 1000000000 "" Meeting
check "answers" "$(rows)" "$(printf '%s\n' \
    '2001-09-09T01:46:41+00:00,afk,,' \
    '2001-09-09T01:46:41+00:00,lunch,,' \
    '2001-09-09T01:46:41+00:00,focus,Minecraft,Meeting' \
    '2001-09-09T01:46:41+00:00,focus,Minecraft,External' \
    '2001-09-09T01:46:41+00:00,focus,Minecraft,Bench' \
    '2001-09-09T01:46:41+00:00,focus,Minecraft,fixing the printer' \
    '2001-09-09T01:46:41+00:00,focus,Unnamed,Meeting')"
rm -rf "$TELEMETRY_DIR"

# carrying on gives the spell to what was last in focus before it, and nothing
# when nothing was
"$ROOT/scripts/telemetry.sh" answer 1000000000 Minecraft c
check "nothing to carry on" "$(rows 2>/dev/null)" ""
mkdir -p "$TELEMETRY_DIR"
printf '%s\n' 'timestamp,event,session,context' \
    '2001-09-09T01:40:00+00:00,focus,Minecraft,"notes, drafts"' \
    '2001-09-09T01:45:00+00:00,focus,Cyberpunk,feat/chrome' \
    '2001-09-09T01:46:40+00:00,idle,,' \
    '2001-09-09T01:50:00+00:00,focus,Roblox,main' \
    '2001-09-09T01:20:00+00:00,focus,Roblox,filled in late' > "$TELEMETRY_DIR/2001-09.csv"
"$ROOT/scripts/telemetry.sh" answer 1000000000 Minecraft continue
check "carry on" "$(rows | tail -n 1)" '2001-09-09T01:46:41+00:00,focus,Cyberpunk,feat/chrome'
printf '%s\n' 'timestamp,event,session,context' '2001-08-31T23:59:00+00:00,focus,Minecraft,"notes, drafts"' > "$TELEMETRY_DIR/2001-08.csv"
"$ROOT/scripts/telemetry.sh" answer 999302400 Roblox c
check "carry on from the month before" "$(rows | tail -n 1)" '2001-09-01T00:00:01+00:00,focus,Minecraft,"notes, drafts"'
rm -rf "$TELEMETRY_DIR"

# a client goes quiet, says it is stepping away, comes back and says it was AFK
export TELEMETRY_TICK=1 TELEMETRY_IDLE_AFTER=3
tmux -f /dev/null new-session -d -s Work
tmux run-shell "$ROOT/telemetry.tmux"
python3 - <<'PY' &
import os, pty, select, time
pid, fd = pty.fork()
if pid == 0:
    os.execvp("tmux", ["tmux", "attach", "-t", "Work"])
start = time.time()
script = [(6, b"n"), (8, b"x"), (9, b"a\r")]
while time.time() < start + 13:
    if script and time.time() > start + script[0][0]:
        os.write(fd, script.pop(0)[1])
    # a quiet screen sends nothing, so wait only a little for it
    if select.select([fd], [], [], 0.2)[0]:
        try:
            os.read(fd, 65536)
        except OSError:
            break
PY
sleep 12

events=$(rows | cut -d, -f2 | grep -v focus)
check "away, then AFK" "$events" "$(printf '%s\n' idle afk)"
idle=$(rows | grep ',idle,' | cut -d, -f1)
afk=$(rows | grep ',afk,' | cut -d, -f1)
if [[ -z "$idle" || "$(date -d "$afk" +%s)" != "$(( $(date -d "$idle" +%s) + 1 ))" ]]; then
    echo "idle: AFK should begin a second after the spell, got $idle and $afk"
    failed=1
fi
check "back at work after answering" "$(rows | tail -n 1 | cut -d, -f2,3)" "focus,Work"

(( failed )) && exit 1
echo "idle: ok"
