#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, to a CSV file for
# the week that is rolled into one for its year each Monday; timesheet turns the
# records into a report

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"
HEADER="timestamp,event,session,context"
LABELS="Bench Research Hardware"
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
  meeting <PANE>  start or end a project meeting in the pane's session
  external <PANE> start or end an external meeting, which marks every session;
                  ending one asks what it was about
  topic <SINCE>   in a popup, ask what the external meeting begun then was about
  roll            add the weeks before this one to their year files, with repeated
                  records folded together; the first record of a week does this

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
    local file="$LOGS/week.csv" week rolled=""
    mkdir -p "$LOGS"
    printf -v week '%(%G-W%V)T' -1
    read -r rolled 2>/dev/null < "$LOGS/.week"
    [[ "$rolled" == "$week" ]] || roll
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
# the pane shows
label() {
    local window=${1,,} label
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

# <variable> <session>: the session's name without a meeting's marks
unmark() {
    local _unmarked=$2
    [[ "$_unmarked" == "<"?*">" || "$_unmarked" == "["?*"]" ]] && _unmarked=${_unmarked:1:-1}
    printf -v "$1" '%s' "$_unmarked"
}

# only the pane in focus counts, so background panes finishing a command don't
record_pane() {
    local active session name path ignore trunk window labels context
    # read folds runs of tabs together, so fields that may be empty carry a leading =
    IFS=$'\t' read -r active session path ignore trunk labels window < <(tmux display-message -p -t "$1" \
        '#{&&:#{session_attached},#{&&:#{window_active},#{pane_active}}}	#{session_name}	#{pane_current_path}	=#{@telemetry-ignore}	=#{@telemetry-trunk}	=#{@telemetry-labels}	=#{window_name}')
    ignore=${ignore#=} trunk=${trunk#=} labels=${labels#=} window=${window#=}
    unmark name "$session"

    [[ "$active" == 1 && -n "$name" && " $ignore " != *" $name "* ]] || return 0

    # lunch and time away belong to no project, so they go down without the session
    case "${window,,}" in
        lunch|afk)
            append "${window,,}" "" ""
            return
            ;;
    esac

    # an external meeting belongs to no project, so it goes down as a session of its own
    case "$session" in
        "["?*"]")
            append focus External Meeting
            return
            ;;
        "<"?*">")
            append focus "$name" Meeting
            return
            ;;
    esac

    context=$(label "$window" "$labels") || context=$(work "$path" "$trunk")
    append focus "$name" "$context"
}

# a popup on an ignored session is a client of its own, so closing it leaves another attached
record_away() {
    local ignore session
    ignore=$(option ignore)
    while read -r session; do
        unmark session "$session"
        [[ " $ignore " == *" $session "* ]] || return 0
    done < <(tmux list-clients -F '#{client_session}')
    append away "" ""
}


#-------------------------------------------------------------------------------
# rolling the week over

# The records go to week.csv, and the first record of each ISO week moves those
# from before its Monday into a file for their year, such as 2026.csv. A writer
# claims week.csv by renaming it to .rolling.<pid>.csv, and timesheet reads any
# such file left by a roll cut short, so no record goes missing on the way.

# <file>...: the records in time order with repeats folded together. Within a day,
# a record the same as the ones either side of it says nothing new, as timesheet
# counts a run of them as one stretch from its first to its last.
fold() {
    awk -F, '
        # RFC 3339 as seconds since 1970, as awk has no dates of its own
        function epoch(s,   y, m, d, offset, era, yoe, doe) {
            y = substr(s, 1, 4) + 0; m = substr(s, 6, 2) + 0; d = substr(s, 9, 2) + 0
            offset = 0
            if (substr(s, 20, 1) != "Z") {
                offset = substr(s, 21, 2) * 3600 + substr(s, 24, 2) * 60
                if (substr(s, 20, 1) == "-") offset = -offset
            }
            if (m <= 2) y--
            era = int(y / 400); yoe = y - era * 400
            doe = yoe * 365 + int(yoe / 4) - int(yoe / 100) + int((153 * ((m + 9) % 12) + 2) / 5) + d - 1
            return (era * 146097 + doe - 719468) * 86400 + substr(s, 12, 2) * 3600 + substr(s, 15, 2) * 60 + substr(s, 18, 2) - offset
        }
        $1 ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T/ { printf "%d\t%d\t%s\n", epoch($1), NR, $0 }
    ' "$@" | sort -t $'\t' -k1,1n -k2,2n | cut -f 3- | awk '
        function rest(s) { return substr(s, index(s, ",") + 1) }
        { line[NR] = $0 }
        END {
            # a topic names a meeting rather than marking a moment, so runs pass through it
            for (i = 1; i <= NR; i++) {
                topic = rest(line[i]) ~ /^topic,/
                if (!topic) {
                    for (next_one = i + 1; next_one <= NR && rest(line[next_one]) ~ /^topic,/; next_one++) {}
                    same_before = previous && substr(line[previous], 1, 10) == substr(line[i], 1, 10) && rest(line[previous]) == rest(line[i])
                    same_after = next_one <= NR && substr(line[next_one], 1, 10) == substr(line[i], 1, 10) && rest(line[next_one]) == rest(line[i])
                    previous = i
                    if (same_before && same_after) continue
                }
                if (line[i] == printed) continue
                print line[i]
                printed = line[i]
            }
        }
    '
}

# a lock left by a roll that never finished is taken over after ten minutes
lock_roll() {
    local lock="$LOGS/.rolling.lock"
    mkdir "$lock" 2>/dev/null && return
    [[ -n "$(find "$lock" -maxdepth 0 -mmin +10 2>/dev/null)" ]] || return 1
    rmdir "$lock" 2>/dev/null
    mkdir "$lock" 2>/dev/null
}

# <file> <first byte>: rows a writer added to a claimed file after it was read
# go back to the week
put_back() {
    local week="$LOGS/week.csv"
    (( $(wc -c < "$1") >= $2 )) || return 0
    [ -s "$week" ] || echo "$HEADER" >> "$week"
    tail -c +"$2" "$1" | grep -v '^timestamp' >> "$week"
}

roll() {
    local week monday claimed="$LOGS/.rolling.$$.csv" file year rows rest sizes=()
    mkdir -p "$LOGS"
    lock_roll || return 0
    printf -v week '%(%G-W%V)T' -1
    # Monday at noon, so a clock change in the week can't move it a day
    monday=$(date -d "$(printf '%(%Y-%m-%d)T' -1) 12:00 $(( $(printf '%(%u)T' -1) - 1 )) days ago" +%Y-%m-%d)
    [[ -s "$LOGS/week.csv" ]] && mv "$LOGS/week.csv" "$claimed"
    echo "$week" > "$LOGS/.week"

    shopt -s nullglob
    local claims=("$LOGS"/.rolling.*.csv)
    shopt -u nullglob
    if (( ${#claims[@]} )); then
        for file in "${claims[@]}"; do
            sizes+=("$(wc -c < "$file")")
        done
        rows=$(for i in "${!claims[@]}"; do head -c "${sizes[i]}" "${claims[i]}"; echo; done | grep -v '^timestamp' | grep .)
        # this week's records stay in the week
        rest=$(awk -v monday="$monday" '/^[0-9]/ && substr($0, 1, 10) >= monday' <<< "$rows")
        if [[ -n "$rest" ]]; then
            [ -s "$LOGS/week.csv" ] || echo "$HEADER" >> "$LOGS/week.csv"
            echo "$rest" >> "$LOGS/week.csv"
        fi
        for year in $(awk -v monday="$monday" '/^[0-9]/ && substr($0, 1, 10) < monday { print substr($0, 1, 4) }' <<< "$rows" | sort -u); do
            file="$LOGS/$year.csv"
            {
                echo "$HEADER"
                fold <(awk -v monday="$monday" -v year="$year" 'substr($0, 1, 10) < monday && substr($0, 1, 4) == year' <<< "$rows") \
                    "$([ -r "$file" ] && echo "$file" || echo /dev/null)"
            } > "$file.tmp" && mv "$file.tmp" "$file"
        done
        for i in "${!claims[@]}"; do
            put_back "${claims[i]}" $(( sizes[i] + 1 ))
            rm -f "${claims[i]}"
        done
    fi
    rmdir "$LOGS/.rolling.lock"
}


#-------------------------------------------------------------------------------
# meetings

# A session in a meeting wears it in its name, <name> for a project meeting and
# [name] for an external one, so the status line shows it and recording reads it.

# <mark>: every session renamed into an external meeting, given [], or out of any meeting
mark_sessions() {
    local id name bare
    while IFS=$'\t' read -r id name; do
        unmark bare "$name"
        [[ -n "$1" ]] && bare="[$bare]"
        [[ "$bare" == "$name" ]] || tmux rename-session -t "$id" "$bare"
    done < <(tmux list-sessions -F '#{session_id}	#{session_name}')
}

external_meeting() {
    tmux list-sessions -F '#{session_name}' | grep -q '^\[.*\]$'
}

# an external meeting keeps when it began, to ask once it ends what it was about
external_begins() {
    mkdir -p "$LOGS"
    date +%s > "$LOGS/.external"
}

# <pane>: the external meeting has ended, so ask what it was about, once, however
# many hooks see it end
external_ends() {
    local held="$LOGS/.external.$$" since session client
    mv "$LOGS/.external" "$held" 2> /dev/null || return 0
    since=$(< "$held")
    rm -f "$held"
    session=$(tmux display-message -p -t "$1" '#{session_id}')
    client=$(tmux list-clients -t "$session" -F '#{client_name}' | head -n 1)
    [[ -n "$client" ]] || return 0
    tmux display-popup -c "$client" -E -w 66 -h 9 -T ' telemetry ' -e "TELEMETRY_DIR=$LOGS" \
        "$(printf '%q ' "$SELF" topic "$since")" &
}

# <since>: runs in the popup. What the meeting was about goes down as its work in
# place of Meeting, recorded as a topic for the meeting that began then.
topic() {
    local about
    printf '\n  What was the external meeting from %s about?\n\n' "$(clock "$1")"
    printf '  Enter alone leaves it as Meeting\n\n  > '
    read -re about
    [[ -n "$about" ]] && write_row "$(stamp "$1")" topic External "$about"
    return 0
}

# <pane>: a project meeting starts or ends in the pane's session; one starting
# ends any other meeting, external or not
toggle_meeting() {
    local id name
    IFS=$'\t' read -r id name < <(tmux display-message -p -t "$1" '#{session_id}	#{session_name}')
    [[ -n "$id" ]] || return 0
    if [[ "$name" == "<"?*">" ]]; then
        tmux rename-session -t "$id" "${name:1:-1}"
    else
        external_meeting && external_ends "$1"
        mark_sessions ""
        unmark name "$name"
        tmux rename-session -t "$id" "<$name>"
    fi
    record_pane "$1"
}

# <pane>: an external meeting starts or ends for every session, ending any project meeting
toggle_external() {
    if external_meeting; then
        mark_sessions ""
        external_ends "$1"
        record_pane "$1"
    else
        mark_sessions "[]"
        record_pane "$1"
        # after its first record, so the topic is dated within the meeting
        external_begins
    fi
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

# <pid>: the command it runs; slim Linux images have no ps, but always have /proc
command_of() {
    if [[ -r "/proc/$1/cmdline" ]]; then
        tr '\0' ' ' < "/proc/$1/cmdline"
    else
        ps -o args= -p "$1" 2>/dev/null
    fi
}

# One watcher at a time, and a reload of the config takes over from the one
# running, so an updated plugin is the one keeping watch. Each look leaves the
# time in .alive, so a look that comes far later than it should, after the
# machine slept or the watcher was gone with tmux, knows where the gap began.
watch() {
    local pidfile="$LOGS/.watcher" alive="$LOGS/.alive" last now old
    mkdir -p "$LOGS"
    old=$(cat "$pidfile" 2>/dev/null)
    # the pid may since have gone to some other process, so only a watcher is stopped
    if [[ "$old" =~ ^[0-9]+$ && "$(command_of "$old")" == *"$(basename "$SELF") watch"* ]]; then
        kill "$old" 2>/dev/null
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

# the latest keypress from any client not on an ignored session, then the client,
# its session and the window it has in focus
latest_keypress() {
    local ignore name session bare activity window best=0 who=""
    ignore=$(option ignore)
    while IFS=$'\t' read -r name session activity window; do
        unmark bare "$session"
        [[ " $ignore " == *" $bare "* ]] && continue
        if (( activity > best )); then
            best=$activity
            who="$name"$'\t'"$session"$'\t'"$window"
        fi
    done < <(tmux list-clients -F '#{client_name}	#{client_session}	#{client_activity}	#{window_name}')
    [[ -n "$who" ]] && printf '%s\t%s\n' "$best" "$who"
}

# <now>: after @telemetry-idle minutes without a key pressed, 5 unless set, ask
# whether anyone is there, once for each quiet spell. A window named afk or lunch
# already says nobody is, and a meeting says where the time is going, so there is
# nothing to ask.
check_quiet() {
    local now=$1 after activity client session window idle
    idle=$(option idle)
    after=${TELEMETRY_IDLE_AFTER:-$(( ${idle:-5} * 60 ))}
    IFS=$'\t' read -r activity client session window < <(latest_keypress) || return 0
    case "${window,,}" in
        afk|lunch)
            return 0
            ;;
    esac
    [[ "$session" == "<"?*">" || "$session" == "["?*"]" ]] && return 0
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
    local since=$1 session=$2 client=$3 key where about
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
    # l, m, a label or anything typed still work, as before, without crowding the list
    printf '\n  Unattended since %s. Log activity:\n\n' "$(clock "$since")"
    printf '  [a] AFK\n'
    printf '  [e] External Meeting\n'
    printf '  [p] Project Meeting\n'
    printf '  [c] Continue Previous\n\n  > '
    read -re where
    case "${where,,}" in
        e|external|!meeting)
            clear
            printf '\n  What was the external meeting about?\n\n  Enter alone leaves it as Meeting\n\n  > '
            read -re about
            [[ -n "$about" ]] && where="e $about"
            ;;
    esac
    answer "$since" "$session" "$where"

    # the break ends now, rather than at the next prompt
    record_pane "$(tmux display-message -p -c "$client" '#{pane_id}')"
}

# <since>: the session and work last in focus before a moment, as the records
# hold them, so they can be written again as they are. A spell can begin just
# after a week, a month or a year does, so the files before are looked through too.
last_focus() {
    local before month file files=()
    before=$(stamp "$1")
    for month in "$(date -d "$(printf '%(%Y-%m-01)T' "$1") -1 day" +%Y-%m)" "$(printf '%(%Y-%m)T' "$1")"; do
        for file in "$LOGS/$month.csv" "$LOGS/${month:0:4}.csv"; do
            [[ -r "$file" ]] && files+=("$file")
        done
    done
    for file in "$LOGS/week.csv" "$LOGS"/.rolling.*.csv; do
        [[ -r "$file" ]] && files+=("$file")
    done
    (( ${#files[@]} )) || return 1
    # an answer is written long after the spell it fills, so no file is in time order
    awk -v before="$before" '
        substr($0, 26, 7) == ",focus," && substr($0, 1, 25) <= before && substr($0, 1, 25) >= best {
            best = substr($0, 1, 25)
            found = substr($0, 33)
        }
        END { if (found == "") exit 1; print found }
    ' "${files[@]}"
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
        p|m|meeting)
            write_row "$at" focus "$session" Meeting
            ;;
        e|external|!meeting)
            # an external meeting belongs to no project, as with prefix M
            write_row "$at" focus External Meeting
            ;;
        "e "?*|"external "?*)
            # and what it was about stands in for Meeting
            write_row "$at" focus External "${where#* }"
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

    meeting)
        toggle_meeting "$1"
        ;;

    external)
        toggle_external "$1"
        ;;

    topic)
        topic "$1"
        ;;

    roll)
        roll
        ;;

    ""|--help|-h|help)
        show_help
        ;;

    *)
        echo -e "\033[31mUnknown action: $ACTION\033[0m"
        exit 1
        ;;
esac
