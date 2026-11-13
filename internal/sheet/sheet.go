// Package sheet turns records into what was worked on, when, and for how long.
package sheet

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

// Rules shape the working day: when it starts, and the lunch left out of it.
type Rules struct {
	DayStart   time.Duration
	LunchStart time.Duration
	LunchEnd   time.Duration
}

// DefaultRules start the day at nine and leave out twelve to one.
var DefaultRules = Rules{DayStart: 9 * time.Hour, LunchStart: 12 * time.Hour, LunchEnd: 13 * time.Hour}

// NoLunch sits past the end of any day, so nothing is ever cut for it.
const NoLunch = 24 * time.Hour

// ParseClock reads HH:MM as time since midnight.
func ParseClock(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a time of day as HH:MM", s)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// ParseLunch reads HH:MM-HH:MM, or none.
func ParseLunch(s string) (start, end time.Duration, err error) {
	if strings.EqualFold(s, "none") {
		return NoLunch, NoLunch, nil
	}
	from, to, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("%q is not a lunch as HH:MM-HH:MM, or none", s)
	}
	if start, err = ParseClock(from); err != nil {
		return 0, 0, err
	}
	if end, err = ParseClock(to); err != nil {
		return 0, 0, err
	}
	if end <= start {
		return 0, 0, fmt.Errorf("lunch %q ends before it starts", s)
	}
	return start, end, nil
}

// Work is what a stretch of time went on: a session, and a branch or folder in it.
type Work struct {
	Session string
	Context string
}

// AFK is the label for time away from the keyboard: shown, but never counted.
const AFK = "AFK"

// Counted says whether the time goes into totals, which AFK time doesn't.
func (w Work) Counted() bool { return !strings.EqualFold(w.Context, AFK) }

// Entry is one stretch of work in a day, as time since that day's midnight.
type Entry struct {
	Work
	Start time.Duration
	End   time.Duration
}

// Duration is how long the entry lasted.
func (e Entry) Duration() time.Duration { return e.End - e.Start }

// Day is one day's entries, in order.
type Day struct {
	Date    record.Date
	Entries []Entry
}

// Total is all the time counted in a day, which leaves out AFK.
func (d Day) Total() time.Duration {
	var total time.Duration
	for _, e := range d.Entries {
		if e.Counted() {
			total += e.Duration()
		}
	}
	return total
}

// Away is the AFK time in a day.
func (d Day) Away() time.Duration {
	var away time.Duration
	for _, e := range d.Entries {
		if !e.Counted() {
			away += e.Duration()
		}
	}
	return away
}

// Build splits records into days and turns each day into entries.
func Build(records []record.Record, rules Rules) []Day {
	var days []Day
	for start := 0; start < len(records); {
		date := record.DateOf(records[start].Time)
		end := start
		for end < len(records) && record.DateOf(records[end].Time).Equal(date) {
			end++
		}
		if entries := buildDay(records[start:end], rules); len(entries) > 0 {
			days = append(days, Day{Date: date, Entries: entries})
		}
		start = end
	}
	return days
}

type run struct {
	work  Work
	away  bool
	start time.Duration
}

func clock(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
}

// Records in a row for the same work make one run, which lasts until the next run
// starts; the last run of the day ends at the day's last record. Lunch always ends
// a run, so work after it starts afresh at the first record back.
func buildDay(records []record.Record, rules Rules) []Entry {
	var runs []run
	var last time.Duration
	for i, rec := range records {
		t := clock(rec.Time)
		w := Work{Session: rec.Session, Context: rec.Context}
		away := rec.Event == record.Away
		intoLunch := i > 0 && last < rules.LunchStart && t >= rules.LunchStart
		if len(runs) == 0 || intoLunch || runs[len(runs)-1].away != away || runs[len(runs)-1].work != w {
			runs = append(runs, run{work: w, away: away, start: t})
		}
		last = t
	}

	var entries []Entry
	started := false
	for i, r := range runs {
		if r.away {
			continue
		}
		start, end := r.start, last
		if i+1 < len(runs) {
			end = runs[i+1].start
		}

		// the day starts at DayStart, even if the terminal came out later
		if !started && start < rules.LunchStart {
			start = rules.DayStart
		}
		start = max(start, rules.DayStart)
		if start >= rules.LunchStart && start < rules.LunchEnd {
			start = rules.LunchEnd
		}
		if start < rules.LunchStart && end > rules.LunchStart {
			end = rules.LunchStart
		}
		if end <= start {
			continue
		}
		started = true
		entries = append(entries, Entry{Work: r.work, Start: start, End: end})
	}
	return entries
}

// Total is the time spent on one piece of work across days.
type Total struct {
	Work
	Duration time.Duration
}

// Totals sums each piece of work across the days, longest first, with AFK after
// everything counted.
func Totals(days []Day) []Total {
	totals := sum(days, true, func(w Work) Work { return w })
	sort.SliceStable(totals, func(i, j int) bool { return totals[i].Counted() && !totals[j].Counted() })
	return totals
}

// Sessions sums the time counted in each session across the days, longest first.
func Sessions(days []Day) []Total {
	return sum(days, false, func(w Work) Work { return Work{Session: w.Session} })
}

func sum(days []Day, withAway bool, key func(Work) Work) []Total {
	index := map[Work]int{}
	var totals []Total
	for _, day := range days {
		for _, e := range day.Entries {
			if !withAway && !e.Counted() {
				continue
			}
			k := key(e.Work)
			i, ok := index[k]
			if !ok {
				i = len(totals)
				index[k] = i
				totals = append(totals, Total{Work: k})
			}
			totals[i].Duration += e.Duration()
		}
	}
	sort.SliceStable(totals, func(i, j int) bool { return totals[i].Duration > totals[j].Duration })
	return totals
}

// Grand is the time counted across all the days.
func Grand(days []Day) time.Duration {
	var total time.Duration
	for _, d := range days {
		total += d.Total()
	}
	return total
}

// Away is the AFK time across all the days.
func Away(days []Day) time.Duration {
	var away time.Duration
	for _, d := range days {
		away += d.Away()
	}
	return away
}

// Hours shows a duration as H:MM, the way a timesheet does.
func Hours(d time.Duration) string {
	minutes := int(d.Round(time.Minute) / time.Minute)
	return fmt.Sprintf("%d:%02d", minutes/60, minutes%60)
}

// Clock shows time since midnight as HH:MM.
func Clock(d time.Duration) string {
	minutes := int(d / time.Minute)
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
