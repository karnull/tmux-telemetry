#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, one file a day;
# timesheet turns the records into a report

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"


show_help() {
cat <<'EOF'
Usage: telemetry [action]

Actions:
  record <PANE>   log the session and branch or folder of a tmux pane
  away            log that tmux was detached

Notes:
  - record and away are run by the tmux and shell hooks, not by hand
  - timesheet reads the records and reports the time spent
EOF
}

option() {
    tmux show-option -gqv "@telemetry-$1" 2>/dev/null
}

#-------------------------------------------------------------------------------
# recording

append() {
    local now
    printf -v now '%(%F %H:%M)T' -1
    mkdir -p "$LOGS"
    printf '%s\t%s\t%s\n' "${now#* }" "$1" "$2" >> "$LOGS/${now% *}.tsv"
}

# only the pane in focus counts, so background panes finishing a command don't
record_pane() {
    local active session path ignore context
    IFS=$'\t' read -r active session path ignore < <(tmux display-message -p -t "$1" \
        '#{&&:#{session_attached},#{&&:#{window_active},#{pane_active}}}	#{session_name}	#{pane_current_path}	#{@telemetry-ignore}')

    [[ "$active" == 1 && -n "$session" && " $ignore " != *" $session "* ]] || return 0

    context=$(git -C "$path" symbolic-ref --short -q HEAD 2>/dev/null) ||
        context=$(git -C "$path" rev-parse --short HEAD 2>/dev/null) ||
        context="${path/#$HOME/\~}"

    append "$session" "$context"
}

# a popup on an ignored session is a client of its own, so closing it leaves another attached
record_away() {
    local ignore session
    ignore=$(option ignore)
    while read -r session; do
        [[ " $ignore " == *" $session "* ]] || return 0
    done < <(tmux list-clients -F '#{client_session}')
    append "" ""
}


#-------------------------------------------------------------------------------
# special commands

case "$ACTION" in
    record)
        record_pane "$1"
        ;;

    away)
        record_away
        ;;

    ""|--help|-h|help)
        show_help
        ;;

    *)
        echo -e "\033[31mUnknown action: $ACTION\033[0m"
        exit 1
        ;;
esac
