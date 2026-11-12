package period

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

// The forms each kind of period can take, by example, for errors and help.
const (
	DayForms   = "11/04/2026, 11/04, 2026-04-11, today, yesterday or a weekday"
	WeekForms  = "25,2026, 2026-W25, W25, 25 or any day in it"
	MonthForms = "04/2026, 2026-04, April 2026, april, 4 or any day in it"
)

var (
	dayMonthYear = regexp.MustCompile(`^(\d{1,2})[/.](\d{1,2})[/.](\d{4})$`)
	dayMonth     = regexp.MustCompile(`^(\d{1,2})[/.](\d{1,2})$`)
	isoDay       = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	weekYear     = regexp.MustCompile(`^[wW]?(\d{1,2})\s*[,/ ]\s*(\d{4})$`)
	isoWeek      = regexp.MustCompile(`^(\d{4})-?[wW](\d{1,2})$`)
	bareWeek     = regexp.MustCompile(`^[wW](\d{1,2})$`)
	monthYear    = regexp.MustCompile(`^(\d{1,2})[/.](\d{4})$`)
	isoMonth     = regexp.MustCompile(`^(\d{4})-(\d{1,2})$`)
	number       = regexp.MustCompile(`^\d{1,2}$`)
)

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// calendarDay builds a day, refusing ones that roll over, such as 31/04
func calendarDay(year, month, day int) (record.Date, bool) {
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return d, d.Year() == year && int(d.Month()) == month && d.Day() == day
}

// ParseDay reads a day in any of DayForms. Day and month go the British way round.
func ParseDay(s string, today record.Date) (record.Date, error) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	switch lower {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}

	// a weekday is the latest one, today included
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		name := strings.ToLower(wd.String())
		if lower == name || lower == name[:3] {
			return today.AddDate(0, 0, -((int(today.Weekday()) - int(wd) + 7) % 7)), nil
		}
	}

	var y, m, d int
	switch {
	case dayMonthYear.MatchString(s):
		p := dayMonthYear.FindStringSubmatch(s)
		d, m, y = atoi(p[1]), atoi(p[2]), atoi(p[3])
	case dayMonth.MatchString(s):
		p := dayMonth.FindStringSubmatch(s)
		d, m, y = atoi(p[1]), atoi(p[2]), today.Year()
	case isoDay.MatchString(s):
		p := isoDay.FindStringSubmatch(s)
		y, m, d = atoi(p[1]), atoi(p[2]), atoi(p[3])
	default:
		return record.Date{}, fmt.Errorf("%q is not a day; try one such as %s", s, DayForms)
	}
	day, ok := calendarDay(y, m, d)
	if !ok {
		return record.Date{}, fmt.Errorf("%q is not a day on the calendar", s)
	}
	return day, nil
}

// weeksIn is how many ISO weeks a year has: 53 when 28 December falls in week 53.
func weeksIn(year int) int {
	_, w := time.Date(year, 12, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w
}

// ISOWeek is week n of an ISO year, which starts on the Monday of the week holding 4 January.
func ISOWeek(year, week int) (Period, error) {
	if week < 1 || week > weeksIn(year) {
		return Period{}, fmt.Errorf("%d has weeks 1 to %d, not %d", year, weeksIn(year), week)
	}
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	return Containing(Week, jan4.AddDate(0, 0, 7*(week-1))), nil
}

// ParseWeek reads a week in any of WeekForms. A week number alone is in this ISO year.
func ParseWeek(s string, today record.Date) (Period, error) {
	s = strings.TrimSpace(s)
	thisYear, _ := today.ISOWeek()
	switch {
	case weekYear.MatchString(s):
		p := weekYear.FindStringSubmatch(s)
		return ISOWeek(atoi(p[2]), atoi(p[1]))
	case isoWeek.MatchString(s):
		p := isoWeek.FindStringSubmatch(s)
		return ISOWeek(atoi(p[1]), atoi(p[2]))
	case bareWeek.MatchString(s):
		return ISOWeek(thisYear, atoi(bareWeek.FindStringSubmatch(s)[1]))
	case number.MatchString(s):
		return ISOWeek(thisYear, atoi(s))
	}
	if day, err := ParseDay(s, today); err == nil {
		return Containing(Week, day), nil
	}
	return Period{}, fmt.Errorf("%q is not a week; try one such as %s", s, WeekForms)
}

func monthNamed(s string) (time.Month, bool) {
	s = strings.ToLower(s)
	for m := time.January; m <= time.December; m++ {
		name := strings.ToLower(m.String())
		if s == name || (len(s) >= 3 && strings.HasPrefix(name, s)) {
			return m, true
		}
	}
	return 0, false
}

func month(year, m int, s string) (Period, error) {
	if m < 1 || m > 12 {
		return Period{}, fmt.Errorf("%q is not a month on the calendar", s)
	}
	return Containing(Month, time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC)), nil
}

// ParseMonth reads a month in any of MonthForms. A month without its year is in this year.
func ParseMonth(s string, today record.Date) (Period, error) {
	s = strings.TrimSpace(s)
	switch {
	case monthYear.MatchString(s):
		p := monthYear.FindStringSubmatch(s)
		return month(atoi(p[2]), atoi(p[1]), s)
	case isoMonth.MatchString(s):
		p := isoMonth.FindStringSubmatch(s)
		return month(atoi(p[1]), atoi(p[2]), s)
	case number.MatchString(s):
		return month(today.Year(), atoi(s), s)
	}

	fields := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(fields) >= 1 && len(fields) <= 2 {
		if m, ok := monthNamed(fields[0]); ok {
			year := today.Year()
			if len(fields) == 2 {
				if _, err := strconv.Atoi(fields[1]); err != nil || len(fields[1]) != 4 {
					return Period{}, fmt.Errorf("%q is not a month; try one such as %s", s, MonthForms)
				}
				year = atoi(fields[1])
			}
			return month(year, int(m), s)
		}
	}
	if day, err := ParseDay(s, today); err == nil {
		return Containing(Month, day), nil
	}
	return Period{}, fmt.Errorf("%q is not a month; try one such as %s", s, MonthForms)
}

// Guess reads a day, week or month from its form alone, for when no option says
// which. A bare number could be any of them, so it is refused.
func Guess(s string, today record.Date) (Period, error) {
	s = strings.TrimSpace(s)
	if number.MatchString(s) {
		return Period{}, fmt.Errorf("%q could be a day, a week or a month; say which with --day, --week or --month", s)
	}
	if day, err := ParseDay(s, today); err == nil {
		return Containing(Day, day), nil
	}
	// NN/YYYY reads as a month here; with --week it is a week
	if monthYear.MatchString(s) {
		return ParseMonth(s, today)
	}
	if weekYear.MatchString(s) || isoWeek.MatchString(s) || bareWeek.MatchString(s) {
		return ParseWeek(s, today)
	}
	if p, err := ParseMonth(s, today); err == nil {
		return p, nil
	}
	return Period{}, fmt.Errorf("%q is not a day, week or month I can read; try a day such as %s; a week such as %s; or a month such as %s",
		s, DayForms, WeekForms, MonthForms)
}
