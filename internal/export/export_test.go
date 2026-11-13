package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

var days = []sheet.Day{{
	Date: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
	Entries: []sheet.Entry{
		{Work: sheet.Work{Session: "Roblox", Context: "release-1.0"}, Start: 9 * time.Hour, End: 12 * time.Hour},
		{Work: sheet.Work{Session: "Cyberpunk", Context: "notes, drafts"}, Start: 13 * time.Hour, End: 14 * time.Hour},
		{Work: sheet.Work{Session: "Cyberpunk", Context: "AFK"}, Start: 14 * time.Hour, End: 14*time.Hour + 30*time.Minute},
	},
}}

func TestCSV(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, days); err != nil {
		t.Fatal(err)
	}
	want := "date,start,end,session,context,minutes,counted\n" +
		"2026-11-10,09:00,12:00,Roblox,release-1.0,180,true\n" +
		"2026-11-10,13:00,14:00,Cyberpunk,\"notes, drafts\",60,true\n" +
		"2026-11-10,14:00,14:30,Cyberpunk,AFK,30,false\n"
	if b.String() != want {
		t.Errorf("got\n%s", b.String())
	}
}

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	p := period.Containing(period.Week, days[0].Date)
	if err := JSON(&b, p, days); err != nil {
		t.Fatal(err)
	}
	var got jsonReport
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Minutes != 240 || got.AFKMinutes != 30 || got.Period.First != "2026-11-09" || got.Period.Unit != "week" {
		t.Errorf("got %+v", got)
	}
	if got.Sessions[0].Session != "Roblox" || got.Sessions[0].Share != 0.75 {
		t.Errorf("sessions: %+v", got.Sessions)
	}
	if last := got.Totals[len(got.Totals)-1]; last.Counted || last.Share != 0 || last.Minutes != 30 {
		t.Errorf("AFK should be listed, uncounted, with no share: %+v", last)
	}
	if len(got.Sessions) != 2 || got.Sessions[1].Minutes != 60 {
		t.Errorf("Cyberpunk's session total should leave out AFK: %+v", got.Sessions)
	}
	if !strings.Contains(b.String(), `"context": "notes, drafts"`) {
		t.Errorf("context lost: %s", b.String())
	}
}
