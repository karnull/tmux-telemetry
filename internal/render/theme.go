// Package render draws reports as styled text: tables, a pie chart and timelines.
package render

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Theme decides how sessions stand apart: by colour, or by glyph without it.
type Theme struct {
	Colour  bool
	palette map[string]int
	order   []string
}

// the terminal's own ANSI colours, so reports follow its theme
var ansi = []int{4, 2, 3, 5, 6, 1, 12, 10, 11, 13, 14, 9}

// glyphs tell sessions apart where colour isn't available
var glyphs = []string{"█", "▓", "▒", "░", "▚", "#", "%", "*", "+", "=", "~", ":"}

// NewTheme gives each session its own colour and glyph, the longest first.
func NewTheme(colour bool, sessions []sheet.Total) Theme {
	t := Theme{Colour: colour, palette: map[string]int{}}
	for _, s := range sessions {
		t.palette[s.Session] = len(t.order)
		t.order = append(t.order, s.Session)
	}
	return t
}

func (t Theme) index(session string) int {
	if i, ok := t.palette[session]; ok {
		return i
	}
	return len(t.order)
}

// Colour of a session's marks.
func (t Theme) Color(session string) color.Color {
	return lipgloss.ANSIColor(ansi[t.index(session)%len(ansi)])
}

// Glyph that fills a session's marks.
func (t Theme) Glyph(session string) string {
	if t.Colour {
		return "█"
	}
	return glyphs[t.index(session)%len(glyphs)]
}

// Mark is a short swatch for a session, for legends and tables.
func (t Theme) Mark(session string) string {
	if !t.Colour {
		return t.Glyph(session)
	}
	return lipgloss.NewStyle().Foreground(t.Color(session)).Render("■")
}

// Paint styles text in a session's colour.
func (t Theme) Paint(session, text string) string {
	if !t.Colour {
		return text
	}
	return lipgloss.NewStyle().Foreground(t.Color(session)).Render(text)
}

func (t Theme) style() lipgloss.Style { return lipgloss.NewStyle() }

// Heading is text set bold, as a report's title is.
func (t Theme) Heading(s string) string { return t.bold(s) }

func (t Theme) bold(s string) string {
	if !t.Colour {
		return s
	}
	return t.style().Bold(true).Render(s)
}

func (t Theme) faint(s string) string {
	if !t.Colour {
		return s
	}
	return t.style().Faint(true).Render(s)
}
