// Package sheet turns records into what was worked on, when, and for how long.
package sheet

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

// Work is what a stretch of time went on: a session, and a branch or folder in it,
// or a lunch break, which belongs to no session.
type Work struct {
	Session string
	Context string
	Break   bool
}

// AFK is the label for time away from the keyboard: shown, but never counted.
const AFK = "AFK"

// LunchBreak names a lunch break where a session would go.
const LunchBreak = "Lunch break"

// Settle is how long a stretch has to last to count as somewhere.
const Settle = time.Minute

// passingThrough says whether the work is only on the way somewhere: the home
// folder, recorded as no folder, or as ~ by older versions.
func (w Work) passingThrough() bool { return !w.Break && (w.Context == "" || w.Context == "~") }

// IsAFK says whether the time was spent away from the keyboard.
func (w Work) IsAFK() bool { return !w.Break && strings.EqualFold(w.Context, AFK) }

// Counted says whether the time goes into totals, which AFK and lunch don't.
func (w Work) Counted() bool { return !w.Break && !w.IsAFK() }

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
func (d Day) Away() time.Duration { return d.sum(func(w Work) bool { return w.IsAFK() }) }

// Breaks is the lunch break time in a day.
func (d Day) Breaks() time.Duration { return d.sum(func(w Work) bool { return w.Break }) }

func (d Day) sum(include func(Work) bool) time.Duration {
	var total time.Duration
	for _, e := range d.Entries {
		if include(e.Work) {
			total += e.Duration()
		}
	}
	return total
}

// Build splits records into days and turns each day into entries. A day runs from
// its first record to its last, whatever the clock says.
func Build(records []record.Record) []Day {
	var days []Day
	for start := 0; start < len(records); {
		date := record.DateOf(records[start].Time)
		end := start
		for end < len(records) && record.DateOf(records[end].Time).Equal(date) {
			end++
		}
		if entries := buildDay(records[start:end]); len(entries) > 0 {
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
// starts; the last run of the day ends at the day's last record. A run at home, or
// one shorter than Settle, hands its start to the next run that settles, and is
// dropped when tmux is detached or the day ends first.
func buildDay(records []record.Record) []Entry {
	var runs []run
	var last time.Duration
	for _, rec := range records {
		t := clock(rec.Time)
		w := Work{Session: rec.Session, Context: rec.Context}
		if rec.Event == record.Lunch {
			w = Work{Session: LunchBreak, Break: true}
		}
		away := rec.Event == record.Away
		if len(runs) == 0 || runs[len(runs)-1].away != away || runs[len(runs)-1].work != w {
			runs = append(runs, run{work: w, away: away, start: t})
		}
		last = t
	}

	var entries []Entry
	pending, waiting := time.Duration(0), false
	for i, r := range runs {
		end := last
		if i+1 < len(runs) {
			end = runs[i+1].start
		}
		if r.away {
			waiting = false
			continue
		}
		if r.work.passingThrough() || end-r.start < Settle {
			if !waiting {
				pending, waiting = r.start, true
			}
			continue
		}

		start := r.start
		if waiting {
			start, waiting = pending, false
		}
		// a hop away and straight back leaves one stretch, not two
		if n := len(entries); n > 0 && entries[n-1].Work == r.work && entries[n-1].End == start {
			entries[n-1].End = end
			continue
		}
		entries = append(entries, Entry{Work: r.work, Start: start, End: end})
	}
	return entries
}

// Total is the time spent on one piece of work across days.
type Total struct {
	Work
	Duration time.Duration
}

// Totals sums each piece of work across the days, kept together by session: the
// sessions longest first, and in each its work longest first, then its time not
// counted. Anything with no counted time at all comes last.
func Totals(days []Day) []Total {
	totals := sum(days, true, func(w Work) Work { return w })
	rank := map[string]int{}
	for i, s := range Sessions(days) {
		rank[s.Session] = i
	}
	group := func(t Total) int {
		if r, ok := rank[t.Session]; ok {
			return r
		}
		return len(rank)
	}
	sort.SliceStable(totals, func(i, j int) bool {
		if gi, gj := group(totals[i]), group(totals[j]); gi != gj {
			return gi < gj
		}
		return totals[i].Counted() && !totals[j].Counted()
	})
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

// Breaks is the lunch break time across all the days.
func Breaks(days []Day) time.Duration {
	var breaks time.Duration
	for _, d := range days {
		breaks += d.Breaks()
	}
	return breaks
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
