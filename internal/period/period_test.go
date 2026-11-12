package period

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestContaining(t *testing.T) {
	tests := []struct {
		unit        Unit
		on          time.Time
		first, last time.Time
	}{
		{Week, date(2026, 11, 12), date(2026, 11, 9), date(2026, 11, 15)},
		{Week, date(2026, 11, 15), date(2026, 11, 9), date(2026, 11, 15)},
		{Week, date(2026, 11, 9), date(2026, 11, 9), date(2026, 11, 15)},
		{Month, date(2026, 2, 14), date(2026, 2, 1), date(2026, 2, 28)},
		{Day, date(2026, 11, 12), date(2026, 11, 12), date(2026, 11, 12)},
	}
	for _, tt := range tests {
		p := Containing(tt.unit, tt.on)
		if !p.First.Equal(tt.first) || !p.Last.Equal(tt.last) {
			t.Errorf("%s of %s: got %s to %s", tt.unit, tt.on.Format(time.DateOnly), p.First.Format(time.DateOnly), p.Last.Format(time.DateOnly))
		}
	}
}

func TestShift(t *testing.T) {
	if p := Containing(Month, date(2026, 3, 31)).Shift(-1); !p.First.Equal(date(2026, 2, 1)) || !p.Last.Equal(date(2026, 2, 28)) {
		t.Errorf("the month before March: got %s to %s", p.First, p.Last)
	}
	if p := Containing(Week, date(2027, 1, 1)).Shift(-1); !p.First.Equal(date(2026, 12, 21)) {
		t.Errorf("the week before New Year's Day 2027 starts %s", p.First)
	}
	if p := Containing(Day, date(2026, 3, 1)).Shift(-1); !p.First.Equal(date(2026, 2, 28)) {
		t.Errorf("the day before 01/03 is %s", p.First)
	}
	r, _ := Between(date(2026, 11, 1), date(2026, 11, 10))
	if p := r.Shift(1); !p.First.Equal(date(2026, 11, 11)) || !p.Last.Equal(date(2026, 11, 20)) {
		t.Errorf("a ten-day range steps ten days: got %s to %s", p.First, p.Last)
	}
}

func TestBetweenRejectsBackwards(t *testing.T) {
	if _, err := Between(date(2026, 11, 10), date(2026, 11, 1)); err == nil {
		t.Error("a range that ends before it starts should be refused")
	}
}

func TestTitle(t *testing.T) {
	if got := Containing(Week, date(2026, 11, 12)).Title(); got != "Week 46, Mon 09/11 to Sun 15/11/2026" {
		t.Errorf("got %q", got)
	}
	if got := Containing(Month, date(2026, 11, 12)).Title(); got != "November 2026" {
		t.Errorf("got %q", got)
	}
	if got := Containing(Day, date(2026, 11, 10)).Title(); got != "Tuesday 10/11/2026" {
		t.Errorf("got %q", got)
	}
}
