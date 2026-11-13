package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// DetailLine is one line of a detail. A line about narrower work carries its
// selection, so the TUI can open it when the line is clicked.
type DetailLine struct {
	Text   string
	Select *sheet.Selection
}

// DetailText joins a detail's lines, for printing.
func DetailText(lines []DetailLine) string {
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.Text
	}
	return strings.Join(texts, "\n") + "\n"
}

// Detail is every stretch spent on a selection across the days: how long in all,
// what it was made of, and each day's stretches beside a timeline marking them.
func Detail(t Theme, days []sheet.Day, sel sheet.Selection, width int) []DetailLine {
	var lines []DetailLine
	add := func(s string) { lines = append(lines, DetailLine{Text: s}) }
	pick := func(s string, w sheet.Work) {
		chosen := sheet.Select(w)
		lines = append(lines, DetailLine{Text: s, Select: &chosen})
	}

	sel = sheet.Resolve(days, sel)
	add(t.bold(t.Paint(sel.Session, sel.Title())))
	picked := sheet.Filter(days, sel)
	if len(picked) == 0 {
		add("")
		add(t.faint("Nothing recorded for it in this period."))
		return lines
	}

	spent := sheet.Spent(picked)
	summary := fmt.Sprintf("%s over %d day(s), in %d stretch(es)", sheet.Hours(spent), len(picked), sheet.Stretches(picked))
	if sel.Break != sheet.NoBreak {
		summary += ", not counted"
	} else if grand := sheet.Grand(days); grand > 0 {
		summary += fmt.Sprintf(", %.0f%% of the %s counted", 100*spent.Seconds()/grand.Seconds(), sheet.Hours(grand))
	}
	longest, on := sheet.Longest(picked)
	add(t.faint(summary))
	add(t.faint(fmt.Sprintf("Longest %s, %s - %s on %s; first in at %s, last out at %s",
		sheet.Hours(longest.Duration()), sheet.Clock(longest.Start), sheet.Clock(longest.End), on.Date.Format("Mon 02/01"),
		sheet.Clock(earliest(picked)), sheet.Clock(latest(picked)))))

	if !sel.Single() {
		add("")
		parts := sheet.Totals(picked)
		nameWidth := 0
		for _, p := range parts {
			nameWidth = max(nameWidth, len([]rune(partName(sel, p.Work))))
		}
		for _, p := range parts {
			share := p.Duration.Seconds() / spent.Seconds()
			pick(fmt.Sprintf("  %s %s  %6s  %s %3.0f%%", t.Mark(p.Session), pad(t.Paint(p.Session, partName(sel, p.Work)), nameWidth),
				sheet.Hours(p.Duration), pad(Bar(t, p.Session, share, 10), 10), 100*share), p.Work)
		}
	}

	const label = len("Mon 09/11 ")
	const total = len(" 10:00")
	barWidth := max(10, width-label-total-2)
	span := SpanOf(picked)
	add("")
	add(strings.Repeat(" ", label+1) + Axis(t, span, barWidth))
	for _, d := range picked {
		add(d.Date.Format("Mon 02/01") + " " + t.faint("│") + Highlight(t, dayOf(days, d), sel, span, barWidth) +
			t.faint("│") + " " + sheet.Hours(d.Spent()))
		for _, e := range d.Entries {
			text := fmt.Sprintf("%s%s - %s  %6s", strings.Repeat(" ", label+1), sheet.Clock(e.Start), sheet.Clock(e.End), sheet.Hours(e.Duration()))
			if sel.Single() {
				add(text)
				continue
			}
			pick(text+"  "+t.Mark(e.Session)+" "+t.Paint(e.Session, partName(sel, e.Work)), e.Work)
		}
	}
	return lines
}

// partName is what tells one part of a selection from another: the work in a
// session, the session for a branch across sessions, or both otherwise.
func partName(sel sheet.Selection, w sheet.Work) string {
	switch {
	case w.Break != sheet.NoBreak:
		return w.Session
	case sel.Session != "":
		return w.Context
	case sel.Context != "":
		return w.Session
	}
	return w.Session + " · " + w.Context
}

// dayOf finds the whole day a filtered day came from, so the rest of it shows faintly.
func dayOf(days []sheet.Day, picked sheet.Day) sheet.Day {
	for _, d := range days {
		if d.Date.Equal(picked.Date) {
			return d
		}
	}
	return picked
}

func earliest(days []sheet.Day) time.Duration {
	first := 24 * time.Hour
	for _, d := range days {
		first = min(first, d.Entries[0].Start)
	}
	return first
}

func latest(days []sheet.Day) time.Duration {
	var last time.Duration
	for _, d := range days {
		last = max(last, d.Entries[len(d.Entries)-1].End)
	}
	return last
}
