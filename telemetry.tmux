#!/usr/bin/env bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TELEMETRY="$CURRENT_DIR/scripts/telemetry.sh"

# shells started in tmux find the shell hook and the command through this
tmux set-environment -g TELEMETRY_HOME "$CURRENT_DIR"

# a fixed index, so reloading the config replaces the hooks rather than stacking them
# window-renamed catches a window renamed to a label, with no prompt needed
for hook in client-session-changed session-window-changed after-select-pane client-attached window-renamed; do
    tmux set-hook -g "$hook[42]" "run-shell -b '$TELEMETRY record #{pane_id}'"
done
tmux set-hook -g 'client-detached[42]' "run-shell -b '$TELEMETRY away'"

# prefix m and M, unless set otherwise, start and end a project or an external meeting
meeting_key=$(tmux show-option -gqv @telemetry-meeting-key)
external_key=$(tmux show-option -gqv @telemetry-external-key)
tmux bind-key "${meeting_key:-m}" run-shell -b "$TELEMETRY meeting #{pane_id}"
tmux bind-key "${external_key:-M}" run-shell -b "$TELEMETRY external #{pane_id}"

# the watcher has a session of its own, so it outlives tmux long enough to say when it stopped
TELEMETRY_SOCKET=$(tmux display-message -p '#{socket_path}') setsid "$TELEMETRY" watch > /dev/null 2>&1 < /dev/null &
