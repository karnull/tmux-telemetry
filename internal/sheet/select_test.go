package sheet

import (
	"testing"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

func detailDays() []Day {
	recs := records("2026-11-12",
		"09:00 Minecraft a",
		"10:00 Cyberpunk b",
		"11:00 Minecraft a",
		"12:35 lunch",
		"13:00 Minecraft a",
		"17:45 Roblox a",
		"18:00 Roblox a",
	)
	recs[3] = record.Record{Time: recs[3].Time, Event: record.Lunch}
	return Build(recs)
}

func TestSelectionsPickTheirWork(t *testing.T) {
	days := detailDays()
	tests := []struct {
		sel  Selection
		want string
	}{
		{Selection{Session: "Minecraft", Context: "a"}, "09:00 - 10:00 Minecraft a\n11:00 - 12:35 Minecraft a\n13:00 - 17:45 Minecraft a"},
		{Selection{Session: "minecraft"}, "09:00 - 10:00 Minecraft a\n11:00 - 12:35 Minecraft a\n13:00 - 17:45 Minecraft a"},
		{Selection{Context: "a"}, "09:00 - 10:00 Minecraft a\n11:00 - 12:35 Minecraft a\n13:00 - 17:45 Minecraft a\n17:45 - 18:00 Roblox a"},
		{Selection{Break: LunchBreak}, "12:35 - 13:00 Lunch break "},
		{SelectNamed("lunch", ""), "12:35 - 13:00 Lunch break "},
	}
	for _, tt := range tests {
		got := Filter(days, tt.sel)
		if len(got) != 1 || timetable(got[0]) != tt.want {
			t.Errorf("%s:\ngot\n%v\nwant\n%s", tt.sel.Title(), got, tt.want)
		}
	}
	if Filter(days, Selection{Session: "Nowhere"}) != nil {
		t.Error("a session never worked on should leave no days")
	}
}

func TestDetailFigures(t *testing.T) {
	days := Filter(detailDays(), Selection{Session: "Minecraft", Context: "a"})
	if Hours(Spent(days)) != "7:20" || Stretches(days) != 3 {
		t.Errorf("spent %s in %d stretches, want 7:20 in 3", Hours(Spent(days)), Stretches(days))
	}
	if e, _ := Longest(days); e.Start != 13*time.Hour || e.End != 17*time.Hour+45*time.Minute {
		t.Errorf("longest %s - %s, want 13:00 - 17:45", Clock(e.Start), Clock(e.End))
	}
}

func TestSelectionTitles(t *testing.T) {
	for sel, want := range map[Selection]string{
		{Session: "Minecraft", Context: "main"}: "Minecraft · main",
		{Session: "Minecraft"}:                  "Minecraft",
		{Context: "main"}:                       "main · every session",
		{Break: AFKBreak}:                       "AFK",
	} {
		if got := sel.Title(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestResolveSpellsAsRecorded(t *testing.T) {
	got := Resolve(detailDays(), Selection{Session: "minecraft", Context: "A"})
	if got.Session != "Minecraft" || got.Context != "a" {
		t.Errorf("got %+v", got)
	}
}
