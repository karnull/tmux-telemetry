// Package period works out which days a report covers.
package period

import (
	"fmt"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

// Unit is the length of a period.
type Unit int

const (
	Day Unit = iota
	Week
	Month
	Range
)

func (u Unit) String() string {
	return [...]string{"day", "week", "month", "range"}[u]
}

// Period is the days from First to Last, inclusive.
type Period struct {
	Unit  Unit
	First record.Date
	Last  record.Date
}

// Today is the current day by the local clock.
func Today() record.Date { return record.DateOf(time.Now()) }

// Containing is the day, week or month a date falls in. Weeks run Monday to Sunday.
func Containing(unit Unit, date record.Date) Period {
	switch unit {
	case Week:
		monday := date.AddDate(0, 0, -((int(date.Weekday()) + 6) % 7))
		return Period{Unit: Week, First: monday, Last: monday.AddDate(0, 0, 6)}
	case Month:
		first := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
		return Period{Unit: Month, First: first, Last: first.AddDate(0, 1, -1)}
	default:
		return Period{Unit: Day, First: date, Last: date}
	}
}

// Between is a custom range, which steps by its own length.
func Between(first, last record.Date) (Period, error) {
	if last.Before(first) {
		return Period{}, fmt.Errorf("the range ends on %s, before it starts on %s",
			last.Format("02/01/2006"), first.Format("02/01/2006"))
	}
	return Period{Unit: Range, First: first, Last: last}, nil
}

// Shift moves the period by n of itself: -1 is the one before.
func (p Period) Shift(n int) Period {
	switch p.Unit {
	case Week:
		return Containing(Week, p.First.AddDate(0, 0, 7*n))
	case Month:
		return Containing(Month, p.First.AddDate(0, n, 0))
	case Range:
		length := p.Days()
		return Period{Unit: Range, First: p.First.AddDate(0, 0, length*n), Last: p.Last.AddDate(0, 0, length*n)}
	default:
		return Containing(Day, p.First.AddDate(0, 0, n))
	}
}

// Days is how many days the period covers.
func (p Period) Days() int {
	return int(p.Last.Sub(p.First).Hours()/24) + 1
}

// Contains says whether a date falls in the period.
func (p Period) Contains(date record.Date) bool {
	return !date.Before(p.First) && !date.After(p.Last)
}

// Dates lists every day in the period.
func (p Period) Dates() []record.Date {
	dates := make([]record.Date, 0, p.Days())
	for d := p.First; !d.After(p.Last); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates
}

// Title names the period the way a person would.
func (p Period) Title() string {
	switch p.Unit {
	case Day:
		return p.First.Format("Monday 02/01/2006")
	case Week:
		_, week := p.First.ISOWeek()
		return fmt.Sprintf("Week %d, %s to %s", week, p.First.Format("Mon 02/01"), p.Last.Format("Sun 02/01/2006"))
	case Month:
		return p.First.Format("January 2006")
	default:
		return fmt.Sprintf("%s to %s", p.First.Format("Mon 02/01/2006"), p.Last.Format("Mon 02/01/2006"))
	}
}
