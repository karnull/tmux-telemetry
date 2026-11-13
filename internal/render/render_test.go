package render

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

var sessions = []sheet.Total{
	{Work: sheet.Work{Session: "A"}, Duration: 3 * time.Hour},
	{Work: sheet.Work{Session: "B"}, Duration: time.Hour},
}

func TestPieSharesWithoutColour(t *testing.T) {
	theme := NewTheme(false, sessions)
	pie := Pie(theme, sessions, 10)
	a := strings.Count(pie, theme.Glyph("A"))
	b := strings.Count(pie, theme.Glyph("B"))
	if a == 0 || b == 0 {
		t.Fatalf("both sessions should show:\n%s", pie)
	}
	if share := float64(a) / float64(a+b); share < 0.7 || share > 0.8 {
		t.Errorf("A holds %.2f of the pie, want about 0.75:\n%s", share, pie)
	}
	if got := lipgloss.Height(pie); got != 10 {
		t.Errorf("a radius of 10 should be 10 rows, got %d", got)
	}
}

func TestPieWithColourUsesHalfBlocks(t *testing.T) {
	pie := Pie(NewTheme(true, sessions), sessions, 4)
	if !strings.ContainsAny(pie, "▀▄") {
		t.Errorf("expected half blocks:\n%s", pie)
	}
}

func TestLegend(t *testing.T) {
	legend := Legend(NewTheme(false, sessions), sessions)
	if !strings.Contains(legend, "A    3:00   75%") || !strings.Contains(legend, "B    1:00   25%") {
		t.Errorf("got\n%s", legend)
	}
}

func TestTimelineMarksLunchAndWork(t *testing.T) {
	theme := NewTheme(false, sessions)
	day := sheet.Day{Entries: []sheet.Entry{
		{Work: sheet.Work{Session: "A"}, Start: 9 * time.Hour, End: 12 * time.Hour},
		{Work: sheet.Work{Session: sheet.LunchBreak, Break: true}, Start: 12 * time.Hour, End: 13 * time.Hour},
		{Work: sheet.Work{Session: "B"}, Start: 13 * time.Hour, End: 17 * time.Hour},
	}}
	span := Span{From: 9 * time.Hour, To: 17 * time.Hour}
	got := Timeline(theme, day, span, 8)
	want := strings.Repeat(theme.Glyph("A"), 3) + "·" + strings.Repeat(theme.Glyph("B"), 4)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTotalsTableFitsTheShareColumn(t *testing.T) {
	theme := NewTheme(true, sessions)
	table := Totals(theme, []sheet.Total{
		{Work: sheet.Work{Session: "A", Context: "x"}, Duration: 3 * time.Hour},
		{Work: sheet.Work{Session: "B", Context: "y"}, Duration: time.Hour},
	}, 4*time.Hour)
	widths := map[int]bool{}
	for _, line := range strings.Split(table, "\n") {
		widths[lipgloss.Width(line)] = true
	}
	if len(widths) != 1 {
		t.Errorf("every row of the table should be the same width:\n%s", table)
	}
}

func TestAFKIsShownButNotShared(t *testing.T) {
	theme := NewTheme(false, sessions)
	day := sheet.Day{Entries: []sheet.Entry{
		{Work: sheet.Work{Session: "A", Context: "x"}, Start: 9 * time.Hour, End: 10 * time.Hour},
		{Work: sheet.Work{Session: "A", Context: "AFK"}, Start: 10 * time.Hour, End: 11 * time.Hour},
	}}
	table := Totals(theme, sheet.Totals([]sheet.Day{day}), day.Total())
	if !strings.Contains(table, NotCounted) || !strings.Contains(table, "| AFK ") {
		t.Errorf("AFK should be listed as not counted:\n%s", table)
	}
	for _, line := range strings.Split(table, "\n") {
		if strings.Contains(line, "Total") && !strings.Contains(line, "1:00") {
			t.Errorf("the total should be the hour counted, not two: %s", line)
		}
	}
	span := Span{From: 9 * time.Hour, To: 11 * time.Hour}
	if got := Timeline(theme, day, span, 2); got != theme.Glyph("A")+"z" {
		t.Errorf("timeline got %q", got)
	}
}

func TestSpanCoversALongDay(t *testing.T) {
	day := sheet.Day{Entries: []sheet.Entry{
		{Work: sheet.Work{Session: "A"}, Start: 8*time.Hour + 30*time.Minute, End: 18*time.Hour + 40*time.Minute},
	}}
	if span := SpanOf([]sheet.Day{day}); span.From != 8*time.Hour || span.To != 19*time.Hour {
		t.Errorf("got %s to %s, want 08:00 to 19:00", sheet.Clock(span.From), sheet.Clock(span.To))
	}
}
