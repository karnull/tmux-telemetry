package sheet

import (
	"strings"
	"time"
)

// Selection picks what a detail is about: one piece of work, a whole session, a
// branch or folder in every session, or a break. Empty fields match anything.
type Selection struct {
	Session string
	Context string
	Break   Break
}

// Select picks exactly the given work.
func Select(w Work) Selection {
	if w.Break != NoBreak {
		return Selection{Break: w.Break}
	}
	return Selection{Session: w.Session, Context: w.Context}
}

// SelectNamed reads a session and work as given on the command line, where afk
// and lunch name the breaks.
func SelectNamed(session, context string) Selection {
	switch strings.ToLower(session) {
	case "afk":
		return Selection{Break: AFKBreak}
	case "lunch", "lunch break":
		return Selection{Break: LunchBreak}
	case "unattended":
		return Selection{Break: UnattendedBreak}
	}
	return Selection{Session: session, Context: context}
}

// Matches says whether a piece of work is part of the selection.
func (s Selection) Matches(w Work) bool {
	if s.Break != NoBreak || w.Break != NoBreak {
		return s.Break == w.Break
	}
	return (s.Session == "" || strings.EqualFold(s.Session, w.Session)) &&
		(s.Context == "" || strings.EqualFold(s.Context, w.Context))
}

// Single says whether the selection is one piece of work, rather than several.
func (s Selection) Single() bool {
	return s.Break != NoBreak || (s.Session != "" && s.Context != "")
}

// Title names the selection the way a heading would.
func (s Selection) Title() string {
	switch {
	case s.Break != NoBreak:
		return s.Break.Name()
	case s.Session != "" && s.Context != "":
		return s.Session + " · " + s.Context
	case s.Session != "":
		return s.Session
	case s.Context != "":
		return s.Context + " · every session"
	}
	return "Everything"
}

// Resolve spells a selection's session and work the way they were recorded, as a
// selection typed on the command line matches regardless of case.
func Resolve(days []Day, sel Selection) Selection {
	for _, d := range days {
		for _, e := range d.Entries {
			if e.Break == NoBreak && sel.Matches(e.Work) {
				if sel.Session != "" {
					sel.Session = e.Session
				}
				if sel.Context != "" {
					sel.Context = e.Context
				}
				return sel
			}
		}
	}
	return sel
}

// Filter keeps only the selected entries, and only the days that have any.
func Filter(days []Day, sel Selection) []Day {
	var out []Day
	for _, d := range days {
		var entries []Entry
		for _, e := range d.Entries {
			if sel.Matches(e.Work) {
				entries = append(entries, e)
			}
		}
		if len(entries) > 0 {
			out = append(out, Day{Date: d.Date, Entries: entries})
		}
	}
	return out
}

// Spent is all the time in a day's entries, breaks included.
func (d Day) Spent() time.Duration { return d.sum(func(Work) bool { return true }) }

// Spent is all the time in the days' entries, breaks included.
func Spent(days []Day) time.Duration {
	var total time.Duration
	for _, d := range days {
		total += d.Spent()
	}
	return total
}

// Longest is the longest entry across the days, and the day it was on.
func Longest(days []Day) (Entry, Day) {
	var best Entry
	var on Day
	for _, d := range days {
		for _, e := range d.Entries {
			if e.Duration() > best.Duration() {
				best, on = e, d
			}
		}
	}
	return best, on
}

// Stretches counts the entries across the days.
func Stretches(days []Day) int {
	n := 0
	for _, d := range days {
		n += len(d.Entries)
	}
	return n
}
