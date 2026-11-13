package render

import (
	"strings"
	"time"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Span is the stretch of the day a timeline covers, on whole hours.
type Span struct {
	From time.Duration
	To   time.Duration
}

// SpanOf covers the working day and every entry in the days, on whole hours.
func SpanOf(days []sheet.Day, rules sheet.Rules) Span {
	span := Span{From: rules.DayStart.Truncate(time.Hour), To: rules.DayStart + 8*time.Hour}
	for _, d := range days {
		for _, e := range d.Entries {
			span.From = min(span.From, e.Start.Truncate(time.Hour))
			span.To = max(span.To, e.End)
		}
	}
	if rem := span.To % time.Hour; rem != 0 {
		span.To += time.Hour - rem
	}
	span.To = min(span.To, 24*time.Hour)
	return span
}

// Axis labels the hours above a timeline of the given width.
func Axis(t Theme, span Span, width int) string {
	line := []rune(strings.Repeat(" ", width))
	hours := int((span.To - span.From) / time.Hour)
	step := 1
	for hours/step*6 > width && step < 24 {
		step++
	}
	for h := 0; h <= hours; h += step {
		col := h * width / max(hours, 1)
		label := []rune(sheet.Clock(span.From + time.Duration(h)*time.Hour)[:2])
		if col+len(label) > width {
			col = width - len(label)
		}
		copy(line[col:], label)
	}
	return t.faint(string(line))
}

// Timeline draws a day as a bar, each cell showing the session that held it longest.
// AFK shows as a faint z, and lunch faintly where nothing was worked through it.
func Timeline(t Theme, day sheet.Day, rules sheet.Rules, span Span, width int) string {
	if width < 1 {
		return ""
	}
	cell := (span.To - span.From) / time.Duration(width)
	var b strings.Builder
	for i := 0; i < width; i++ {
		from := span.From + time.Duration(i)*cell
		to := from + cell
		var best sheet.Entry
		held := time.Duration(0)
		for _, e := range day.Entries {
			if overlap := min(to, e.End) - max(from, e.Start); overlap > held {
				best, held = e, overlap
			}
		}
		switch {
		case held > 0 && !best.Counted():
			b.WriteString(t.faint("z"))
		case held > 0:
			b.WriteString(t.Paint(best.Session, t.Glyph(best.Session)))
		case from < rules.LunchEnd && to > rules.LunchStart:
			b.WriteString(t.faint("·"))
		default:
			b.WriteString(" ")
		}
	}
	return b.String()
}

// Timelines stacks a row for each day under one axis, with each day's total.
func Timelines(t Theme, days []sheet.Day, rules sheet.Rules, width int) string {
	const label = len("Mon 09/11 ")
	const total = len(" 10:00")
	barWidth := max(10, width-label-total-2)
	span := SpanOf(days, rules)

	lines := []string{strings.Repeat(" ", label+1) + Axis(t, span, barWidth)}
	for _, d := range days {
		lines = append(lines, d.Date.Format("Mon 02/01")+" "+
			t.faint("│")+Timeline(t, d, rules, span, barWidth)+t.faint("│")+
			" "+sheet.Hours(d.Total()))
	}
	return strings.Join(lines, "\n")
}
