#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, as one CSV file a
# month; timesheet turns the records into a report

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"
HEADER="timestamp,event,session,context"
LABELS="Bench Research Meeting Hardware"


show_help() {
cat <<'EOF'
Usage: telemetry [action]

Actions:
  record <PANE>   log the session, and the window's label or else the branch or folder;
                  a window named lunch or afk logs a break, with no session
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

# quotes a field only when it needs it, as RFC 4180 has it
csv_field() {
    local -n _field=$1
    [[ "$_field" == *[,\"$'\n']* ]] && _field="\"${_field//\"/\"\"}\""
}

# <timestamp> <event> <session> <context>, the timestamp in RFC 3339
write_row() {
    local timestamp=$1 event=$2 session=$3 context=$4
    local file="$LOGS/${timestamp:0:7}.csv"

    csv_field session
    csv_field context
    mkdir -p "$LOGS"
    [ -s "$file" ] || echo "$HEADER" >> "$file"
    printf '%s,%s,%s,%s\n' "$timestamp" "$event" "$session" "$context" >> "$file"
}

# %z gives +hhmm, and RFC 3339 wants +hh:mm
now() {
    local stamp
    printf -v stamp '%(%Y-%m-%dT%H:%M:%S%z)T' -1
    echo "${stamp:0:22}:${stamp:22}"
}

append() {
    write_row "$(now)" "$@"
}

# a window named for an activity, in any case, is what the time went on, whatever
# the pane shows; !meeting is an internal meeting, which goes down as External
label() {
    local window=${1,,} label
    if [[ "$window" == "!meeting" ]]; then
        echo External
        return
    fi
    for label in ${2:-$LABELS}; do
        if [[ "$window" == "${label,,}" ]]; then
            echo "$label"
            return
        fi
    done
    return 1
}

# <path> <trunk>: the branch checked out, or the folder outside git; the trunk
# branch says nothing about the work, so its folder name stands in. The home
# folder is only passed through, so it records nothing, and timesheet gives its
# time to wherever the work goes next.
work() {
    local branch
    [[ "$1" == "$HOME" ]] && return
    if branch=$(git -C "$1" symbolic-ref --short -q HEAD 2>/dev/null); then
        [[ " ${2:-main master} " == *" $branch "* ]] && branch="${1##*/}"
        echo "$branch"
    else
        git -C "$1" rev-parse --short HEAD 2>/dev/null || echo "${1/#$HOME/\~}"
    fi
}

# only the pane in focus counts, so background panes finishing a command don't
record_pane() {
    local active session path ignore trunk window labels context
    # read folds runs of tabs together, so fields that may be empty carry a leading =
    IFS=$'\t' read -r active session path ignore trunk labels window < <(tmux display-message -p -t "$1" \
        '#{&&:#{session_attached},#{&&:#{window_active},#{pane_active}}}	#{session_name}	#{pane_current_path}	=#{@telemetry-ignore}	=#{@telemetry-trunk}	=#{@telemetry-labels}	=#{window_name}')
    ignore=${ignore#=} trunk=${trunk#=} labels=${labels#=} window=${window#=}

    [[ "$active" == 1 && -n "$session" && " $ignore " != *" $session "* ]] || return 0

    # lunch and time away belong to no project, so they go down without the session
    case "${window,,}" in
        lunch|afk)
            append "${window,,}" "" ""
            return
            ;;
    esac

    context=$(label "$window" "$labels") || context=$(work "$path" "$trunk")
    append focus "$session" "$context"
}

# a popup on an ignored session is a client of its own, so closing it leaves another attached
record_away() {
    local ignore session
    ignore=$(option ignore)
    while read -r session; do
        [[ " $ignore " == *" $session "* ]] || return 0
    done < <(tmux list-clients -F '#{client_session}')
    append away "" ""
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
