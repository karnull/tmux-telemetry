package render

import (
	"fmt"
	"strings"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Options choose what a report shows, and how wide it can be.
type Options struct {
	Width      int
	Timetables bool
	Dir        string
}

// Summary is the time counted over the days, and the AFK and lunch left out of it.
func Summary(days []sheet.Day) string {
	s := fmt.Sprintf("%s over %d day(s)", sheet.Hours(sheet.Grand(days)), len(days))
	var left []string
	if away := sheet.Away(days); away > 0 {
		left = append(left, sheet.Hours(away)+" AFK")
	}
	if breaks := sheet.Breaks(days); breaks > 0 {
		left = append(left, sheet.Hours(breaks)+" lunch")
	}
	if len(left) > 0 {
		s += ", not counting " + strings.Join(left, " and ")
	}
	return s
}

// Report is the whole text report: totals, the pie, a timeline for each day, and
// each day's timetable when asked for.
func Report(t Theme, p period.Period, days []sheet.Day, rules sheet.Rules, opts Options) string {
	grand := sheet.Grand(days)
	header := t.bold(p.Title())
	if len(days) == 0 {
		return header + "\n\nNothing recorded in " + opts.Dir + ".\n"
	}

	sections := []string{
		header + "  " + t.faint(Summary(days)),
		Totals(t, sheet.Totals(days), grand),
		PieWithLegend(t, sheet.Sessions(days), 6),
		Timelines(t, days, rules, opts.Width),
	}
	if opts.Timetables {
		for _, d := range days {
			sections = append(sections, t.bold(d.Date.Format("Monday 02/01/2006"))+"\n"+Timetable(t, d))
		}
	}
	return strings.Join(sections, "\n\n") + "\n"
}
