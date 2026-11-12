package render

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Pie draws each session's share of the time as a circle, radius rows tall. With
// colour, each cell holds two half-block pixels, which come out close to square.
func Pie(t Theme, sessions []sheet.Total, radius int) string {
	var total float64
	for _, s := range sessions {
		total += s.Duration.Seconds()
	}
	if total == 0 || radius < 1 {
		return ""
	}

	// slice boundaries as fractions of the turn, clockwise from twelve o'clock
	bounds := make([]float64, len(sessions))
	var running float64
	for i, s := range sessions {
		running += s.Duration.Seconds() / total
		bounds[i] = running
	}
	sliceAt := func(x, y float64) int {
		r := float64(radius)
		dx, dy := x-r, y-r
		if dx*dx+dy*dy > r*r {
			return -1
		}
		turn := math.Atan2(dx, -dy) / (2 * math.Pi)
		if turn < 0 {
			turn++
		}
		for i, b := range bounds {
			if turn < b {
				return i
			}
		}
		return len(bounds) - 1
	}

	var rows []string
	for row := 0; row < radius; row++ {
		var b strings.Builder
		for col := 0; col < 2*radius; col++ {
			x := float64(col) + 0.5
			if !t.Colour {
				i := sliceAt(x, float64(2*row)+1)
				if i < 0 {
					b.WriteString(" ")
				} else {
					b.WriteString(t.Glyph(sessions[i].Session))
				}
				continue
			}
			b.WriteString(halfBlock(t, sessions, sliceAt(x, float64(2*row)+0.5), sliceAt(x, float64(2*row)+1.5)))
		}
		rows = append(rows, b.String())
	}
	return strings.Join(rows, "\n")
}

func halfBlock(t Theme, sessions []sheet.Total, top, bottom int) string {
	switch {
	case top < 0 && bottom < 0:
		return " "
	case top == bottom:
		return lipgloss.NewStyle().Foreground(t.Color(sessions[top].Session)).Render("█")
	case bottom < 0:
		return lipgloss.NewStyle().Foreground(t.Color(sessions[top].Session)).Render("▀")
	case top < 0:
		return lipgloss.NewStyle().Foreground(t.Color(sessions[bottom].Session)).Render("▄")
	default:
		return lipgloss.NewStyle().
			Foreground(t.Color(sessions[top].Session)).
			Background(t.Color(sessions[bottom].Session)).
			Render("▀")
	}
}

// Legend lists each session with its time and share, beside the pie.
func Legend(t Theme, sessions []sheet.Total) string {
	var total float64
	width := 0
	for _, s := range sessions {
		total += s.Duration.Seconds()
		width = max(width, lipgloss.Width(s.Session))
	}
	var lines []string
	for _, s := range sessions {
		share := 0.0
		if total > 0 {
			share = 100 * s.Duration.Seconds() / total
		}
		name := s.Session + strings.Repeat(" ", width-lipgloss.Width(s.Session))
		lines = append(lines, fmt.Sprintf("%s %s  %6s  %3.0f%%", t.Mark(s.Session), t.Paint(s.Session, name), sheet.Hours(s.Duration), share))
	}
	return strings.Join(lines, "\n")
}

// PieWithLegend sets the legend beside the pie, centred on it.
func PieWithLegend(t Theme, sessions []sheet.Total, radius int) string {
	pie := Pie(t, sessions, radius)
	if pie == "" {
		return ""
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, pie, "    ", Legend(t, sessions))
}
