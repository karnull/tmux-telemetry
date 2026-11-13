package sheet

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

var melbourne = time.FixedZone("AEDT", 11*60*60)

// records takes "HH:MM[:SS] session context" lines, with a bare time for away
func records(day string, lines ...string) []record.Record {
	var out []record.Record
	for _, line := range lines {
		fields := strings.SplitN(line, " ", 3)
		layout := "2006-01-02 15:04"
		if len(fields[0]) > 5 {
			layout += ":05"
		}
		t, err := time.ParseInLocation(layout, day+" "+fields[0], melbourne)
		if err != nil {
			panic(err)
		}
		rec := record.Record{Time: t, Event: record.Away}
		if len(fields) == 3 {
			rec = record.Record{Time: t, Event: record.Focus, Session: fields[1], Context: fields[2]}
		}
		out = append(out, rec)
	}
	return out
}

func timetable(day Day) string {
	var lines []string
	for _, e := range day.Entries {
		lines = append(lines, fmt.Sprintf("%s - %s %s %s", Clock(e.Start), Clock(e.End), e.Session, e.Context))
	}
	return strings.Join(lines, "\n")
}

// the Tuesday from the original request, with the day running from its first record
func TestBuildMatchesTheRequest(t *testing.T) {
	days := Build(records("2026-11-10",
		"08:40 Roblox release-1.0",
		"09:20 Roblox release-1.0",
		"11:58 Roblox release-1.0",
		"12:30",
		"13:32 Minecraft chore/get-started",
		"14:00 Minecraft chore/get-started",
		"15:01 Minecraft chore/auto-update",
		"15:13 Cyberpunk feat/dynamic-weather",
		"15:15 Cyberpunk feat/chrome",
		"15:35 Cyberpunk workspace",
		"16:42 Cyberpunk chore/auto-update",
		"17:11 Cyberpunk feat/dynamic-weather",
		"18:30 Cyberpunk feat/dynamic-weather",
	))

	want := strings.Join([]string{
		"08:40 - 12:30 Roblox release-1.0",
		"13:32 - 15:01 Minecraft chore/get-started",
		"15:01 - 15:13 Minecraft chore/auto-update",
		"15:13 - 15:15 Cyberpunk feat/dynamic-weather",
		"15:15 - 15:35 Cyberpunk feat/chrome",
		"15:35 - 16:42 Cyberpunk workspace",
		"16:42 - 17:11 Cyberpunk chore/auto-update",
		"17:11 - 18:30 Cyberpunk feat/dynamic-weather",
	}, "\n")
	if len(days) != 1 {
		t.Fatalf("got %d days", len(days))
	}
	if got := timetable(days[0]); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	totals := map[string]string{}
	for _, total := range Totals(days) {
		totals[total.Session+" "+total.Context] = Hours(total.Duration)
	}
	for work, hours := range map[string]string{
		"Roblox release-1.0":             "3:50",
		"Minecraft chore/get-started":    "1:29",
		"Minecraft chore/auto-update":    "0:12",
		"Cyberpunk feat/dynamic-weather": "1:21",
		"Cyberpunk feat/chrome":          "0:20",
		"Cyberpunk workspace":            "1:07",
		"Cyberpunk chore/auto-update":    "0:29",
	} {
		if totals[work] != hours {
			t.Errorf("%s: got %s, want %s", work, totals[work], hours)
		}
	}
}

func TestDaysSplitAndSessionsSum(t *testing.T) {
	recs := append(records("2026-11-09", "09:00 A x", "10:00 B y", "11:00 B y"),
		records("2026-11-10", "09:00 A z", "11:30 A z")...)
	days := Build(recs)
	if len(days) != 2 {
		t.Fatalf("got %d days", len(days))
	}
	sessions := Sessions(days)
	if sessions[0].Session != "A" || Hours(sessions[0].Duration) != "3:30" {
		t.Errorf("got %+v, want A first with 3:30", sessions[0])
	}
	if Hours(Grand(days)) != "4:30" {
		t.Errorf("grand total %s", Hours(Grand(days)))
	}
}

func TestAFKIsShownButNotCounted(t *testing.T) {
	days := Build(records("2026-11-12",
		"09:00 Minecraft chore/get-started",
		"10:00 Minecraft afk",
		"10:30 Minecraft chore/get-started",
		"11:00 Cyberpunk Meeting",
		"12:00 Cyberpunk Meeting",
	))

	if got, want := timetable(days[0]), strings.Join([]string{
		"09:00 - 10:00 Minecraft chore/get-started",
		"10:00 - 10:30 Minecraft afk",
		"10:30 - 11:00 Minecraft chore/get-started",
		"11:00 - 12:00 Cyberpunk Meeting",
	}, "\n"); got != want {
		t.Errorf("AFK should still show in the timetable:\ngot\n%s\nwant\n%s", got, want)
	}
	if got := Hours(days[0].Total()); got != "2:30" {
		t.Errorf("day total %s, want 2:30 without the AFK half hour", got)
	}
	if got := Hours(days[0].Away()); got != "0:30" {
		t.Errorf("AFK time %s, want 0:30", got)
	}
	for _, s := range Sessions(days) {
		if s.Session == "Minecraft" && Hours(s.Duration) != "1:30" {
			t.Errorf("Minecraft's total %s, want 1:30 without AFK", Hours(s.Duration))
		}
	}
	totals := Totals(days)
	if last := totals[len(totals)-1]; last.Counted() || Hours(last.Duration) != "0:30" {
		t.Errorf("AFK should be listed last, with its time: %+v", last)
	}
	if Hours(Grand(days)) != "2:30" || Hours(Away(days)) != "0:30" {
		t.Errorf("grand %s, away %s", Hours(Grand(days)), Hours(Away(days)))
	}
}

func TestLunchBreakBelongsToNoSession(t *testing.T) {
	recs := records("2026-11-12", "09:00 Minecraft main", "12:10 Minecraft main", "14:00 Minecraft main")
	recs[1] = record.Record{Time: recs[1].Time, Event: record.Lunch}
	days := Build(recs)

	if got, want := timetable(days[0]), "09:00 - 12:10 Minecraft main\n12:10 - 14:00 Lunch break "; got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
	if Hours(days[0].Total()) != "3:10" || Hours(days[0].Breaks()) != "1:50" || days[0].Away() != 0 {
		t.Errorf("total %s, lunch %s, AFK %s", Hours(days[0].Total()), Hours(days[0].Breaks()), Hours(days[0].Away()))
	}
	for _, s := range Sessions(days) {
		if s.Session == LunchBreak {
			t.Error("a lunch break should not be a session in the pie")
		}
	}
}

func TestDaysRunFromTheFirstRecordToTheLast(t *testing.T) {
	days := Build(records("2026-11-12", "08:30 A x", "12:15 A y", "19:00 A y"))
	if got, want := timetable(days[0]), "08:30 - 12:15 A x\n12:15 - 19:00 A y"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if Hours(days[0].Total()) != "10:30" {
		t.Errorf("total %s, want 10:30", Hours(days[0].Total()))
	}
}

func TestHomeHandsItsTimeOn(t *testing.T) {
	recs := records("2026-11-12", "09:00 A ~", "09:20 A feat/x", "10:00 A feat/x")
	recs[0].Context = ""
	days := Build(append(recs, records("2026-11-12", "10:30 A ~", "11:00 A ~")...))
	if got, want := timetable(days[0]), "09:00 - 10:30 A feat/x"; got != want {
		t.Errorf("time at home should go to where the work went next, and home at the end of the day nowhere:\ngot  %q\nwant %q", got, want)
	}
}

func TestShortStopsHandTheirTimeOn(t *testing.T) {
	days := Build(records("2026-11-12",
		"09:00 A x",
		"10:00:00 B y",
		"10:00:30 C z",
		"11:00 C z",
	))
	if got, want := timetable(days[0]), "09:00 - 10:00 A x\n10:00 - 11:00 C z"; got != want {
		t.Errorf("half a minute in B should go to C:\ngot\n%s\nwant\n%s", got, want)
	}
}

func TestAHopAndBackIsOneStretch(t *testing.T) {
	days := Build(records("2026-11-12", "09:00 A x", "10:00 B y", "10:00:20 A x", "11:00 A x"))
	if got, want := timetable(days[0]), "09:00 - 11:00 A x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShortStopsBeforeDetachingAreDropped(t *testing.T) {
	days := Build(records("2026-11-12", "17:00 A x", "18:00 A ~", "18:30", "19:00 B y", "19:00:30 C z", "19:00:40"))
	if got, want := timetable(days[0]), "17:00 - 18:00 A x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
