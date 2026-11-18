#!/usr/bin/env bash

# The first record of a week moves the weeks before it into their year files,
# folding repeats together without changing anything timesheet reports

for tool in go; do
    command -v "$tool" >/dev/null || { echo "roll: skipped, needs $tool"; exit 0; }
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT

export TELEMETRY_DIR="$SCRATCH/logs" TZ=Australia/Sydney
TELEMETRY="$ROOT/scripts/telemetry.sh"
failed=0

check() {
    if [ "$2" == "$3" ]; then
        return
    fi
    echo "roll: $1"
    diff -u <(echo "$3") <(echo "$2")
    failed=1
}

go build -o "$SCRATCH/timesheet" "$ROOT/cmd" || exit 1
report() { "$SCRATCH/timesheet" --dir "$1" --since 01/01/2025 --format csv; }

now=$(date +%Y-%m-%dT%H:%M:%S%:z)
mkdir -p "$TELEMETRY_DIR"
# repeats, a topic, a quiet spell answered late, a clock going back an hour on
# 05/04, a run across midnight, and a week across the new year
printf '%s\n' 'timestamp,event,session,context' \
    '2026-11-13T12:38:38+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:38:38+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:38:54+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:39:00+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:39:24+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:40:10+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:56:48+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:56:48+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:50:00+11:00,idle,,' \
    '2026-11-13T12:50:01+11:00,focus,Orchard,webserver' \
    '2026-11-13T23:50:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T00:20:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T00:40:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T09:30:00+11:00,focus,External,Meeting' \
    '2026-11-14T09:40:00+11:00,focus,External,Meeting' \
    '2026-11-14T10:00:00+11:00,focus,External,Meeting' \
    '2026-11-14T10:05:00+11:00,away,,' \
    '2026-11-14T09:30:00+11:00,topic,External,planning' \
    '2026-04-05T02:30:00+11:00,focus,Orchard,before' \
    '2026-04-05T02:10:00+10:00,focus,Orchard,after' \
    '2026-04-05T02:50:00+10:00,away,,' \
    '2025-12-31T23:00:00+11:00,focus,Orchard,december' \
    '2025-12-31T23:30:00+11:00,focus,Orchard,december' \
    '2025-12-31T23:59:00+11:00,focus,Orchard,december' \
    '2026-01-01T09:00:00Z,focus,Orchard,january' \
    "$now,focus,Orchard,this week" > "$TELEMETRY_DIR/week.csv"
cp -r "$TELEMETRY_DIR" "$SCRATCH/before"
before=$(report "$SCRATCH/before")

"$TELEMETRY" answer "$(date +%s)" Orchard Research
check "the year files" "$(tail -n +2 "$TELEMETRY_DIR/2025.csv" "$TELEMETRY_DIR/2026.csv" | grep -v '^$')" "$(printf '%s\n' \
    "==> $TELEMETRY_DIR/2025.csv <==" \
    '2025-12-31T23:00:00+11:00,focus,Orchard,december' \
    '2025-12-31T23:59:00+11:00,focus,Orchard,december' \
    "==> $TELEMETRY_DIR/2026.csv <==" \
    '2026-01-01T09:00:00Z,focus,Orchard,january' \
    '2026-04-05T02:30:00+11:00,focus,Orchard,before' \
    '2026-04-05T02:10:00+10:00,focus,Orchard,after' \
    '2026-04-05T02:50:00+10:00,away,,' \
    '2026-11-13T12:38:38+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:39:00+11:00,focus,Orchard,phone-app' \
    '2026-11-13T12:39:24+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:40:10+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:50:00+11:00,idle,,' \
    '2026-11-13T12:50:01+11:00,focus,Orchard,webserver' \
    '2026-11-13T12:56:48+11:00,focus,Orchard,webserver' \
    '2026-11-13T23:50:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T00:20:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T00:40:00+11:00,focus,Lighthouse,"notes, drafts"' \
    '2026-11-14T09:30:00+11:00,focus,External,Meeting' \
    '2026-11-14T09:30:00+11:00,topic,External,planning' \
    '2026-11-14T10:00:00+11:00,focus,External,Meeting' \
    '2026-11-14T10:05:00+11:00,away,,')"
check "the week kept" "$(tail -n +2 "$TELEMETRY_DIR/week.csv" | cut -d, -f 2-)" "$(printf '%s\n' \
    'focus,Orchard,this week' 'focus,Orchard,Research')"
check "nothing left aside" "$(ls -A "$TELEMETRY_DIR")" "$(printf '%s\n' .week 2025.csv 2026.csv week.csv)"
# the answer is the only record added, and it falls in this week
check "the same report" "$(report "$TELEMETRY_DIR" | grep -v Research)" "$(echo "$before" | grep -v Research)"

# a second roll changes nothing, and a week left aside by one cut short is picked up
cp "$TELEMETRY_DIR/2026.csv" "$SCRATCH/2026.csv"
printf '%s\n' 'timestamp,event,session,context' '2026-11-12T09:00:00+11:00,focus,Lighthouse,left aside' \
    > "$TELEMETRY_DIR/.rolling.4242.csv"
"$TELEMETRY" roll
check "left aside" "$(diff "$SCRATCH/2026.csv" "$TELEMETRY_DIR/2026.csv" | grep '^[<>]')" \
    '> 2026-11-12T09:00:00+11:00,focus,Lighthouse,left aside'

# writers at once as the week turns over lose nothing
rm -rf "$TELEMETRY_DIR"
mkdir -p "$TELEMETRY_DIR"
echo 2000-W01 > "$TELEMETRY_DIR/.week"
printf '%s\n' 'timestamp,event,session,context' '2026-11-12T09:00:00+11:00,focus,Lighthouse,last week' \
    > "$TELEMETRY_DIR/week.csv"
for i in $(seq 20); do
    "$TELEMETRY" answer "$(date +%s)" Orchard "writer $i" &
done
wait
check "every writer" "$(cat "$TELEMETRY_DIR"/week.csv "$TELEMETRY_DIR"/.rolling.*.csv 2>/dev/null | grep -c writer)" 20
check "last week" "$(grep -h 'last week' "$TELEMETRY_DIR"/2026.csv "$TELEMETRY_DIR"/.rolling.*.csv 2>/dev/null)" \
    '2026-11-12T09:00:00+11:00,focus,Lighthouse,last week'

(( failed )) && exit 1
echo "roll: ok"
