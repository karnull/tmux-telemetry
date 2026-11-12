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

// Report is the whole text report: totals, the pie, a timeline for each day, and
// each day's timetable when asked for.
func Report(t Theme, p period.Period, days []sheet.Day, rules sheet.Rules, opts Options) string {
	grand := sheet.Grand(days)
	header := t.bold(p.Title())
	if len(days) == 0 {
		return header + "\n\nNothing recorded in " + opts.Dir + ".\n"
	}

	sections := []string{
		header + "  " + t.faint(fmt.Sprintf("%s over %d day(s)", sheet.Hours(grand), len(days))),
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
