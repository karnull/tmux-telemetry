package sheet

import "time"

// Averages is a typical day across a period: when work started and stopped, and
// how long went on lunch and AFK. A field is false in Has when no day had it.
type Averages struct {
	Start, Leave time.Duration
	Lunch, AFK   time.Duration
	Has          struct{ Start, Lunch, AFK bool }
}

// Average works out the typical day. Start and leave are the first and last
// counted work of each day that had some; lunch is over the days with a lunch
// break, so a day without one doesn't shorten it; AFK is over every day.
func Average(days []Day) Averages {
	var a Averages
	var worked, lunched int
	for _, d := range days {
		if start, leave, ok := d.Span(); ok {
			a.Start += start
			a.Leave += leave
			worked++
		}
		if lunch := d.Breaks(); lunch > 0 {
			a.Lunch += lunch
			lunched++
		}
		a.AFK += d.Away()
	}
	if worked > 0 {
		a.Start /= time.Duration(worked)
		a.Leave /= time.Duration(worked)
		a.Has.Start = true
	}
	if lunched > 0 {
		a.Lunch /= time.Duration(lunched)
		a.Has.Lunch = true
	}
	if len(days) > 0 {
		a.AFK /= time.Duration(len(days))
		a.Has.AFK = true
	}
	return a
}

// Span is when the day's counted work began and when it ended.
func (d Day) Span() (start, end time.Duration, ok bool) {
	for _, e := range d.Entries {
		if !e.Counted() {
			continue
		}
		if !ok {
			start, ok = e.Start, true
		}
		end = e.End
	}
	return start, end, ok
}
