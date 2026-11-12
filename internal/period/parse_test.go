package period

import (
	"testing"
	"time"
)

// Thursday 12/11/2026, in ISO week 46
var today = day0(2026, 11, 12)

func day0(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseDay(t *testing.T) {
	tests := map[string]time.Time{
		"11/04/2026": day0(2026, 4, 11),
		"1/4/2026":   day0(2026, 4, 1),
		"11.04.2026": day0(2026, 4, 11),
		"11/04":      day0(2026, 4, 11),
		"2026-04-11": day0(2026, 4, 11),
		"today":      today,
		"Yesterday":  day0(2026, 11, 11),
		"thursday":   today,
		"monday":     day0(2026, 11, 9),
		"Fri":        day0(2026, 11, 6),
		"29/02/2028": day0(2028, 2, 29),
	}
	for in, want := range tests {
		got, err := ParseDay(in, today)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q: got %s %v, want %s", in, got.Format(time.DateOnly), err, want.Format(time.DateOnly))
		}
	}
	for _, bad := range []string{"31/04/2026", "29/02/2026", "13/13/2026", "04-11-2026", "soon", ""} {
		if _, err := ParseDay(bad, today); err == nil {
			t.Errorf("%q should not be a day", bad)
		}
	}
}

func TestParseWeek(t *testing.T) {
	tests := map[string]time.Time{
		"25,2026":    day0(2026, 6, 15),
		"25, 2026":   day0(2026, 6, 15),
		"25/2026":    day0(2026, 6, 15),
		"25 2026":    day0(2026, 6, 15),
		"W25,2026":   day0(2026, 6, 15),
		"2026-W25":   day0(2026, 6, 15),
		"2026W25":    day0(2026, 6, 15),
		"w25":        day0(2026, 6, 15),
		"25":         day0(2026, 6, 15),
		"1,2026":     day0(2025, 12, 29),
		"53,2026":    day0(2026, 12, 28),
		"1,2027":     day0(2027, 1, 4),
		"11/04/2026": day0(2026, 4, 6),
		"today":      day0(2026, 11, 9),
	}
	for in, want := range tests {
		got, err := ParseWeek(in, today)
		if err != nil || !got.First.Equal(want) || !got.Last.Equal(want.AddDate(0, 0, 6)) {
			t.Errorf("%q: got %s %v, want the week from %s", in, got.First.Format(time.DateOnly), err, want.Format(time.DateOnly))
		}
	}
	for _, bad := range []string{"0,2026", "54,2026", "53,2027", "week 25", "2026"} {
		if _, err := ParseWeek(bad, today); err == nil {
			t.Errorf("%q should not be a week", bad)
		}
	}
}

func TestParseMonth(t *testing.T) {
	tests := map[string]time.Time{
		"04/2026":    day0(2026, 4, 1),
		"4/2026":     day0(2026, 4, 1),
		"2026-04":    day0(2026, 4, 1),
		"April 2026": day0(2026, 4, 1),
		"apr 2026":   day0(2026, 4, 1),
		"Apr, 2026":  day0(2026, 4, 1),
		"april":      day0(2026, 4, 1),
		"sept":       day0(2026, 9, 1),
		"4":          day0(2026, 4, 1),
		"11/04/2026": day0(2026, 4, 1),
	}
	for in, want := range tests {
		got, err := ParseMonth(in, today)
		if err != nil || !got.First.Equal(want) {
			t.Errorf("%q: got %s %v, want %s", in, got.First.Format(time.DateOnly), err, want.Format(time.DateOnly))
		}
	}
	for _, bad := range []string{"13/2026", "ap", "april 26", "2026-13"} {
		if _, err := ParseMonth(bad, today); err == nil {
			t.Errorf("%q should not be a month", bad)
		}
	}
}

func TestGuess(t *testing.T) {
	tests := []struct {
		in    string
		unit  Unit
		first time.Time
	}{
		{"11/04/2026", Day, day0(2026, 4, 11)},
		{"yesterday", Day, day0(2026, 11, 11)},
		{"25,2026", Week, day0(2026, 6, 15)},
		{"2026-W25", Week, day0(2026, 6, 15)},
		{"W25", Week, day0(2026, 6, 15)},
		{"04/2026", Month, day0(2026, 4, 1)},
		{"2026-04", Month, day0(2026, 4, 1)},
		{"april", Month, day0(2026, 4, 1)},
	}
	for _, tt := range tests {
		got, err := Guess(tt.in, today)
		if err != nil || got.Unit != tt.unit || !got.First.Equal(tt.first) {
			t.Errorf("%q: got %s from %s %v, want %s from %s", tt.in, got.Unit, got.First.Format(time.DateOnly), err, tt.unit, tt.first.Format(time.DateOnly))
		}
	}
	if _, err := Guess("25", today); err == nil {
		t.Error("a bare number should be refused as ambiguous")
	}
}
