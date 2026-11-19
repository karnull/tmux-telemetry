package sheet

import (
	"testing"
	"time"
)

func TestAverage(t *testing.T) {
	at := func(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }
	work := Work{Session: "Orchard", Context: "main"}
	days := []Day{
		{Entries: []Entry{
			{Work: breakWork(AFKBreak), Start: at(8, 0), End: at(8, 30)},
			{Work: work, Start: at(8, 30), End: at(12, 0)},
			{Work: breakWork(LunchBreak), Start: at(12, 0), End: at(13, 0)},
			{Work: work, Start: at(13, 0), End: at(17, 0)},
			{Work: breakWork(AFKBreak), Start: at(17, 0), End: at(17, 30)},
		}},
		{Entries: []Entry{{Work: work, Start: at(9, 30), End: at(18, 0)}}},
		// a day with only a break started nothing, but its AFK still counts
		{Entries: []Entry{{Work: breakWork(AFKBreak), Start: at(10, 0), End: at(10, 30)}}},
	}
	got := Average(days)
	if !got.Has.Start || got.Start != at(9, 0) || got.Leave != at(17, 30) {
		t.Errorf("start and leave: got %s and %s, want 09:00 and 17:30", Clock(got.Start), Clock(got.Leave))
	}
	if !got.Has.Lunch || got.Lunch != time.Hour {
		t.Errorf("lunch over the one day that had it: got %s", Hours(got.Lunch))
	}
	if !got.Has.AFK || got.AFK != 30*time.Minute {
		t.Errorf("AFK over all three days: got %s", Hours(got.AFK))
	}

	// Saturday and Sunday keep other hours, so they leave start and leave as they were
	weekend := []Day{
		{Date: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Entries: []Entry{{Work: work, Start: at(9, 0), End: at(17, 0)}}},
		{Date: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Entries: []Entry{{Work: work, Start: at(13, 0), End: at(15, 0)}}},
		{Date: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), Entries: []Entry{{Work: breakWork(LunchBreak), Start: at(12, 0), End: at(13, 0)}}},
	}
	if got := Average(weekend); got.Start != at(9, 0) || got.Leave != at(17, 0) || !got.Has.Lunch || got.Lunch != time.Hour {
		t.Errorf("weekends: got start %s, leave %s and lunch %s", Clock(got.Start), Clock(got.Leave), Hours(got.Lunch))
	}
	if onlyWeekend := Average(weekend[1:2]); onlyWeekend.Has.Start {
		t.Errorf("a weekend alone has no start or leave, got %+v", onlyWeekend)
	}

	if none := Average(nil); none.Has.Start || none.Has.Lunch || none.Has.AFK {
		t.Errorf("no days should have no averages, got %+v", none)
	}
}
