#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, as one CSV file a
# month; timesheet turns the records into a report

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"
HEADER="timestamp,event,session,context"


show_help() {
cat <<'EOF'
Usage: telemetry [action]

Actions:
  record <PANE>   log the session and branch or folder of a tmux pane
  away            log that tmux was detached
  migrate         convert daily .tsv records from older versions to monthly .csv

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

# only the pane in focus counts, so background panes finishing a command don't
record_pane() {
    local active session path ignore trunk context
    IFS=$'\t' read -r active session path ignore trunk < <(tmux display-message -p -t "$1" \
        '#{&&:#{session_attached},#{&&:#{window_active},#{pane_active}}}	#{session_name}	#{pane_current_path}	#{@telemetry-ignore}	#{@telemetry-trunk}')

    [[ "$active" == 1 && -n "$session" && " $ignore " != *" $session "* ]] || return 0

    # the trunk branch says nothing about the work, so its folder name stands in
    if context=$(git -C "$path" symbolic-ref --short -q HEAD 2>/dev/null); then
        [[ " ${trunk:-main master} " == *" $context "* ]] && context="${path##*/}"
    else
        context=$(git -C "$path" rev-parse --short HEAD 2>/dev/null) ||
            context="${path/#$HOME/\~}"
    fi

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

# a day's .tsv held HH:MM, session and context, with an empty session for away
migrate() {
    local file day time session context offset count=0
    for file in "$LOGS"/*.tsv; do
        [ -e "$file" ] || break
        day=$(basename "$file" .tsv)
        offset=$(date -d "$day 12:00" +%z)
        while IFS=$'\t' read -r time session context; do
            if [ -n "$session" ]; then
                write_row "${day}T$time:00${offset:0:3}:${offset:3}" focus "$session" "$context"
            else
                write_row "${day}T$time:00${offset:0:3}:${offset:3}" away "" ""
            fi
        done < "$file"
        mv "$file" "$file.migrated"
        count=$((count + 1))
    done
    rm -f "$LOGS/telemetry.txt"
    echo "Migrated $count day(s) of records into $LOGS"
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

    migrate)
        migrate
        ;;

    ""|--help|-h|help)
        show_help
        ;;

    *)
        echo -e "\033[31mUnknown action: $ACTION\033[0m"
        exit 1
        ;;
esac
