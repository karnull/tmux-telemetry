#!/usr/bin/env bash

# Logs which tmux session and git branch (or folder) has focus, one file a day,
# and reports the time spent on each

ACTION=$1
shift

LOGS="${TELEMETRY_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/telemetry}"


show_help() {
cat <<'EOF'
Usage: telemetry [action]

Actions:
  week            time spent this week, from Monday
  all             time spent on every recorded day
  record <PANE>   log the session and branch or folder of a tmux pane
  away            log that tmux was detached

Default:
  <no action>     same as week

Notes:
  - record and away are run by the tmux and shell hooks, not by hand
  - every report also rewrites the full log to telemetry.txt beside the records
EOF
}

# tmux may not be running when a report is asked for, so every option has a default
option() {
    local value
    value=$(tmux show-option -gqv "@telemetry-$1" 2>/dev/null)
    echo "${value:-$2}"
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
# reporting

report_day() {
    local file=$1
    local day
    day=$(basename "$file" .tsv)

    date -d "$day" '+%A %d/%m/%Y'
    echo

    awk -F'\t' \
        -v day_start="$DAY_START" -v lunch_start="${LUNCH%-*}" -v lunch_end="${LUNCH#*-}" '
        function minutes(hhmm,    parts) {
            split(hhmm, parts, ":")
            return parts[1] * 60 + parts[2]
        }
        function clock(m) { return sprintf("%02d:%02d", int(m / 60), m % 60) }
        function hours(m) { return sprintf("%d:%02d", int(m / 60), m % 60) }

        BEGIN {
            day_start = minutes(day_start)
            lunch_start = minutes(lunch_start)
            lunch_end = minutes(lunch_end)
        }

        # consecutive records with the same session and context make one run,
        # and lunch always ends a run
        {
            t = minutes($1)
            key = $2 FS $3
            into_lunch = runs && last < lunch_start && t >= lunch_start
            if (!runs || key != run_key[runs] || into_lunch) {
                run_key[++runs] = key
                run_start[runs] = t
            }
            last = t
        }

        END {
            session_width = 13
            context_width = 21

            for (i = 1; i <= runs; i++) {
                if (run_key[i] == FS) continue

                s = run_start[i]
                e = (i < runs) ? run_start[i + 1] : last

                # the day starts at day_start, even if the terminal came out later
                if (!started && s < lunch_start) s = day_start
                if (s < day_start) s = day_start
                if (s >= lunch_start && s < lunch_end) s = lunch_end
                if (s < lunch_start && e > lunch_start) e = lunch_start
                if (e <= s) continue
                started = 1

                split(run_key[i], field, FS)
                n++
                seg_start[n] = s; seg_end[n] = e
                seg_session[n] = field[1]; seg_context[n] = field[2]

                if (!(run_key[i] in total)) order[++keys] = run_key[i]
                total[run_key[i]] += e - s

                if (length(field[1]) > session_width) session_width = length(field[1])
                if (length(field[2]) > context_width) context_width = length(field[2])
            }

            columns = "%-" session_width "s | %-" context_width "s"

            for (i = 1; i <= n; i++)
                printf "%s - %s | " columns " |\n", clock(seg_start[i]), clock(seg_end[i]),
                    seg_session[i], seg_context[i]

            print ""
            print "Total:"
            for (i = 1; i <= keys; i++) {
                split(order[i], field, FS)
                printf columns " | %-5s |\n", field[1], field[2], hours(total[order[i]])
            }
        }' "$file"
}

report() {
    local file first=1
    for file in "$@"; do
        (( first )) || printf '\n%s\n\n' "-------------------------------------------------------"
        first=0
        report_day "$file"
    done
}

this_week() {
    local monday file
    monday=$(date -d "-$(( $(date +%u) - 1 )) days" +%F)
    for file in "$LOGS"/*.tsv; do
        [[ "$(basename "$file" .tsv)" < "$monday" ]] || echo "$file"
    done
}

show_report() {
    local days=("$LOGS"/*.tsv)
    if [ ! -e "${days[0]}" ]; then
        echo -e "\033[33mNothing recorded yet\033[0m"
        return 0
    fi

    DAY_START=$(option day-start 09:00)
    LUNCH=$(option lunch 12:00-13:00)

    report "${days[@]}" > "$LOGS/telemetry.txt"

    if [ "$1" == all ]; then
        cat "$LOGS/telemetry.txt"
    else
        mapfile -t days < <(this_week)
        (( ${#days[@]} )) && report "${days[@]}"
    fi
}


#-------------------------------------------------------------------------------
# special commands

case "$ACTION" in
    ""|week)
        show_report week
        ;;

    all)
        show_report all
        ;;

    record)
        record_pane "$1"
        ;;

    away)
        record_away
        ;;

    --help|-h|help)
        show_help
        ;;

    *)
        echo -e "\033[31mUnknown action: $ACTION\033[0m"
        exit 1
        ;;
esac
