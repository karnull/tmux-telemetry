#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, as one CSV file a
# month; timesheet turns the records into a report

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"
HEADER="timestamp,event,session,context"
LABELS="Bench Research Meeting Hardware"
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

# how often the watcher looks, in seconds; the tests make it shorter
TICK=${TELEMETRY_TICK:-60}

# the watcher outlives the tmux that started it, so it names the server's socket
tmux() {
    command tmux ${TELEMETRY_SOCKET:+-S "$TELEMETRY_SOCKET"} "$@"
}


show_help() {
cat <<'EOF'
Usage: telemetry [action]

Actions:
  record <PANE>   log the session, and the window's label or else the branch or folder;
                  a window named lunch or afk logs a break, with no session
  away            log that tmux was detached
  watch           keep watch while tmux runs: a gap from a restart goes down as
                  AFK, and the day ends when tmux or the machine stops
  resume <FROM> <TO>
                  log a gap between two moments, as seconds since 1970
  ask <SINCE> <SESSION> <CLIENT>
                  in a popup, ask whether anyone is there after a quiet spell,
                  and once they are back, where they were
  answer <SINCE> <SESSION> <ANSWER>
                  log where the time since a quiet spell began went

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
    csv_field session
    csv_field context
    write_raw "$timestamp" "$event" "$session,$context"
}

# <timestamp> <event> <session and context, quoted as the records hold them>
write_raw() {
    local file="$LOGS/${1:0:7}.csv"
    mkdir -p "$LOGS"
    [ -s "$file" ] || echo "$HEADER" >> "$file"
    printf '%s,%s,%s\n' "$1" "$2" "$3" >> "$file"
}

# <seconds since 1970, or -1 for now> as RFC 3339; %z gives +hhmm, and RFC 3339 wants +hh:mm
stamp() {
    local s
    printf -v s '%(%Y-%m-%dT%H:%M:%S%z)T' "$1"
    echo "${s:0:22}:${s:22}"
}

now() {
    stamp -1
}

# <seconds since 1970> as HH:MM
clock() {
    printf '%(%H:%M)T' "$1"
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
# watching

# <from> <to>: nothing ran between the two, as the machine was off or asleep, or
# tmux wasn't running. A gap shorter than @telemetry-restart minutes is a restart,
# which goes down as AFK; a longer one ends the day where the records stopped.
resume() {
    local from=$1 to=$2 restart
    [[ -n "$from" ]] && (( to > from )) || return 0
    restart=$(option restart)
    if (( to - from < ${restart:-15} * 60 )); then
        write_row "$(stamp "$from")" afk "" ""
    else
        write_row "$(stamp "$from")" away "" ""
    fi
}

# One watcher at a time, however often the config is reloaded. Each look leaves
# the time in .alive, so a look that comes far later than it should, after the
# machine slept or the watcher was gone with tmux, knows where the gap began.
watch() {
    local pidfile="$LOGS/.watcher" alive="$LOGS/.alive" last now
    mkdir -p "$LOGS"
    if [[ -s "$pidfile" ]] && kill -0 "$(< "$pidfile")" 2>/dev/null; then
        return 0
    fi
    echo $$ > "$pidfile"

    last=$(cat "$alive" 2>/dev/null)
    while tmux list-sessions > /dev/null 2>&1; do
        now=$(date +%s)
        if [[ -n "$last" ]] && (( now - last > 2 * TICK )); then
            resume "$last" "$now"
        fi
        echo "$now" > "$alive"
        last=$now
        check_quiet "$now"
        sleep "$TICK"
    done

    # tmux has gone, so the day ends here
    append away "" ""
    date +%s > "$alive"
    rm -f "$pidfile"
}


#-------------------------------------------------------------------------------
# quiet spells

# the latest keypress from any client not on an ignored session, then the client and its session
latest_keypress() {
    local ignore name session activity best=0 who=""
    ignore=$(option ignore)
    while IFS=$'\t' read -r name session activity; do
        [[ " $ignore " == *" $session "* ]] && continue
        if (( activity > best )); then
            best=$activity
            who="$name"$'\t'"$session"
        fi
    done < <(tmux list-clients -F '#{client_name}	#{client_session}	#{client_activity}')
    [[ -n "$who" ]] && printf '%s\t%s\n' "$best" "$who"
}

# <now>: after @telemetry-idle minutes without a key pressed, 5 unless set, ask
# whether anyone is there, once for each quiet spell
check_quiet() {
    local now=$1 after activity client session idle
    idle=$(option idle)
    after=${TELEMETRY_IDLE_AFTER:-$(( ${idle:-5} * 60 ))}
    IFS=$'\t' read -r activity client session < <(latest_keypress) || return 0
    (( now - activity >= after )) || return 0
    [[ "$(cat "$LOGS/.asked" 2>/dev/null)" == "$activity" ]] && return 0
    echo "$activity" > "$LOGS/.asked"

    tmux display-popup -c "$client" -E -w 66 -h 11 -T ' telemetry ' -e "TELEMETRY_DIR=$LOGS" \
        "$(printf '%q ' "$SELF" ask "$activity" "$session" "$client")" &
}

# <since> <session> <client>: runs in the popup. No answer in two minutes, or no,
# leaves the time since the last key as UNATTENDED; once someone is back, they
# can say where they were.
ask() {
    local since=$1 session=$2 client=$3 key where
    printf '\n  Still there? Nothing typed since %s.\n\n' "$(clock "$since")"
    printf '  [y] yes, carry on        [n] no, stepping away\n\n'
    printf '  No answer in two minutes counts as away.\n'
    if read -rsn1 -t 120 key && [[ "${key,,}" != n ]]; then
        return
    fi
    write_row "$(stamp "$since")" idle "" ""

    clear
    printf '\n  Away since %s, down as UNATTENDED for now.\n\n  Press any key when back.\n' "$(clock "$since")"
    read -rsn1

    clear
    printf '\n  Where were you since %s?\n\n' "$(clock "$since")"
    printf '  [c] carry on with what you were doing\n'
    printf '  [a] AFK   [l] lunch   [m] Meeting   [e] External\n'
    printf '  or type what it was; Enter alone leaves it UNATTENDED\n\n  > '
    read -re where
    answer "$since" "$session" "$where"

    # the break ends now, rather than at the next prompt
    record_pane "$(tmux display-message -p -c "$client" '#{pane_id}')"
}

# <since>: the session and work last in focus before a moment, as the records
# hold them, so they can be written again as they are. A spell can begin just
# after a month does, so the month before is looked through too.
last_focus() {
    local before month file line best="" found=""
    before=$(stamp "$1")
    for month in "$(date -d "$(printf '%(%Y-%m-01)T' "$1") -1 day" +%Y-%m)" "$(printf '%(%Y-%m)T' "$1")"; do
        file="$LOGS/$month.csv"
        [[ -r "$file" ]] || continue
        # an answer is written long after the spell it fills, so the file isn't in time order
        while IFS= read -r line; do
            [[ "${line:25:7}" == ",focus," ]] || continue
            [[ "${line:0:25}" > "$before" || "${line:0:25}" < "$best" ]] && continue
            best=${line:0:25}
            found=${line:32}
        done < "$file"
    done
    [[ -n "$found" ]] && echo "$found"
}

# <since> <session> <answer>: where the time since a quiet spell began went. It is
# recorded a second after the spell began, so it takes UNATTENDED's place. A label
# goes to the session in focus when the spell began, or Unnamed without one.
answer() {
    local at session=${2:-Unnamed} where=$3 context
    at=$(stamp $(( $1 + 1 )))
    case "${where,,}" in
        "")
            ;;
        c|continue)
            # carrying on with what was in focus, or left UNATTENDED when nothing was
            if context=$(last_focus "$1"); then
                write_raw "$at" focus "$context"
            fi
            ;;
        a|afk)
            write_row "$at" afk "" ""
            ;;
        l|lunch)
            write_row "$at" lunch "" ""
            ;;
        m)
            write_row "$at" focus "$session" Meeting
            ;;
        e)
            write_row "$at" focus "$session" External
            ;;
        *)
            context=$(label "$where" "$(option labels)") || context=$where
            write_row "$at" focus "$session" "$context"
            ;;
    esac
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

    watch)
        watch
        ;;

    resume)
        resume "$1" "$2"
        ;;

    ask)
        ask "$1" "$2" "$3"
        ;;

    answer)
        answer "$1" "$2" "$3"
        ;;

    ""|--help|-h|help)
        show_help
        ;;

    *)
        echo -e "\033[31mUnknown action: $ACTION\033[0m"
        exit 1
        ;;
esac
