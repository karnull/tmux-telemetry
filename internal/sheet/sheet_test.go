package sheet

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/karnull/tmux-telemetry/internal/record"
)

var melbourne = time.FixedZone("AEDT", 11*60*60)

// records takes "HH:MM session context" lines, with a bare time for away
func records(day string, lines ...string) []record.Record {
	var out []record.Record
	for _, line := range lines {
		fields := strings.SplitN(line, " ", 3)
		t, err := time.ParseInLocation("2006-01-02 15:04", day+" "+fields[0], melbourne)
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

// the Tuesday from the original request
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
	), DefaultRules)

	want := strings.Join([]string{
		"09:00 - 12:00 Roblox release-1.0",
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
		"Roblox release-1.0":             "3:00",
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

func TestLunch(t *testing.T) {
	days := Build(records("2026-11-11",
		"09:05 Minecraft chore/get-started",
		"11:00 Minecraft chore/get-started",
		"12:40 Cyberpunk notes",
		"13:50 Cyberpunk notes",
		"14:32 Minecraft chore/get-started",
		"16:00",
		"16:30 Minecraft chore/get-started",
		"17:25 Minecraft chore/get-started",
	), DefaultRules)

	want := strings.Join([]string{
		"09:00 - 12:00 Minecraft chore/get-started",
		"13:00 - 14:32 Cyberpunk notes",
		"14:32 - 16:00 Minecraft chore/get-started",
		"16:30 - 17:25 Minecraft chore/get-started",
	}, "\n")
	if got := timetable(days[0]); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSameWorkCarriesOnAfterLunch(t *testing.T) {
	days := Build(records("2026-11-12",
		"10:00 A x",
		"11:30 A x",
		"13:20 A x",
		"15:00 A x",
	), DefaultRules)
	if got, want := timetable(days[0]), "09:00 - 12:00 A x\n13:20 - 15:00 A x"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestAfternoonStartIsNotStretched(t *testing.T) {
	days := Build(records("2026-11-12", "14:00 A x", "15:00 A x"), DefaultRules)
	if got, want := timetable(days[0]), "14:00 - 15:00 A x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoLunch(t *testing.T) {
	rules := Rules{DayStart: 9 * time.Hour, LunchStart: NoLunch, LunchEnd: NoLunch}
	days := Build(records("2026-11-12", "10:00 A x", "15:00 A x"), rules)
	if got, want := timetable(days[0]), "09:00 - 15:00 A x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDaysSplitAndSessionsSum(t *testing.T) {
	recs := append(records("2026-11-09", "09:00 A x", "10:00 B y", "11:00 B y"),
		records("2026-11-10", "09:00 A z", "11:30 A z")...)
	days := Build(recs, DefaultRules)
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

func TestParseLunch(t *testing.T) {
	if s, e, err := ParseLunch("12:30-13:15"); err != nil || Clock(s) != "12:30" || Clock(e) != "13:15" {
		t.Errorf("got %v %v %v", s, e, err)
	}
	if s, _, err := ParseLunch("none"); err != nil || s != NoLunch {
		t.Errorf("none: got %v %v", s, err)
	}
	for _, bad := range []string{"12:00", "13:00-12:00", "noon-1"} {
		if _, _, err := ParseLunch(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}
