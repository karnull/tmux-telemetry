package render

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

const shareWidth = 12

// NotCounted stands in for the share of AFK time, which totals leave out.
const NotCounted = "not counted"

// Bar is a share of the whole as a bar, to an eighth of a cell.
func Bar(t Theme, session string, share float64, width int) string {
	eighths := int(share*float64(width*8) + 0.5)
	partial := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	full := strings.Repeat(t.Glyph(session), eighths/8)
	bar := full
	if t.Colour {
		bar += partial[eighths%8]
	}
	return t.Paint(session, bar)
}

func newTable(t Theme) *table.Table {
	tbl := table.New().Border(lipgloss.RoundedBorder())
	if !t.Colour {
		return tbl.Border(lipgloss.ASCIIBorder())
	}
	return tbl.BorderStyle(lipgloss.NewStyle().Faint(true))
}

func cellStyle(t Theme, header bool) lipgloss.Style {
	s := lipgloss.NewStyle().Padding(0, 1)
	if header && t.Colour {
		s = s.Bold(true)
	}
	return s
}

// Totals is a table of every piece of work, with its time and share of the whole.
func Totals(t Theme, totals []sheet.Total, grand time.Duration) string {
	tbl := newTable(t).Headers("Session", "Work", "Time", "Share")
	for _, total := range totals {
		if !total.Counted() {
			tbl.Row(t.Mark(total.Session)+" "+t.faint(total.Session), t.faint(total.Context),
				t.faint(sheet.Hours(total.Duration)), t.faint(NotCounted))
			continue
		}
		share := 0.0
		if grand > 0 {
			share = total.Duration.Seconds() / grand.Seconds()
		}
		tbl.Row(
			t.Mark(total.Session)+" "+t.Paint(total.Session, total.Session),
			total.Context,
			sheet.Hours(total.Duration),
			fmt.Sprintf("%s %3.0f%%", pad(Bar(t, total.Session, share, shareWidth), shareWidth), 100*share),
		)
	}
	tbl.Row("", t.bold("Total"), t.bold(sheet.Hours(grand)), "")
	return tbl.StyleFunc(func(row, col int) lipgloss.Style {
		s := cellStyle(t, row == table.HeaderRow)
		if col == 2 {
			s = s.Align(lipgloss.Right)
		}
		return s
	}).String()
}

// Timetable is one day's entries in order: what was being done, and when.
func Timetable(t Theme, day sheet.Day) string {
	tbl := newTable(t).Headers("Time", "Session", "Work", "Hours")
	for _, e := range day.Entries {
		if !e.Counted() {
			tbl.Row(t.faint(sheet.Clock(e.Start)+" - "+sheet.Clock(e.End)), t.Mark(e.Session)+" "+t.faint(e.Session),
				t.faint(e.Context), t.faint(sheet.Hours(e.Duration())))
			continue
		}
		tbl.Row(
			sheet.Clock(e.Start)+" - "+sheet.Clock(e.End),
			t.Mark(e.Session)+" "+t.Paint(e.Session, e.Session),
			e.Context,
			sheet.Hours(e.Duration()),
		)
	}
	tbl.Row("", "", t.bold("Total"), t.bold(sheet.Hours(day.Total())))
	return tbl.StyleFunc(func(row, col int) lipgloss.Style {
		s := cellStyle(t, row == table.HeaderRow)
		if col == 3 {
			s = s.Align(lipgloss.Right)
		}
		return s
	}).String()
}

// pad fills to a visible width, which escape codes would throw off for fmt
func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
