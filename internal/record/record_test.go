package record

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseQuotingAndAway(t *testing.T) {
	got, err := Parse(strings.NewReader(`timestamp,event,session,context
2026-11-10T09:05:00+11:00,focus,Minecraft,chore/get-started
2026-11-10T12:30:00+11:00,away,,
2026-11-10T13:32:00+11:00,focus,Cyberpunk,"~/notes, drafts"
timestamp,event,session,context
2026-11-10T14:00:00+11:00,focus,Cyberpunk,"say ""hi"""
2026-11-10T15:00:00+11:00,lunch,,
2026-11-10T16:00:00+11:00,afk,,
2026-11-10T17:00:00+11:00,idle,,
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 0 || len(got.Records) != 7 {
		t.Fatalf("got %d records, %d skipped", len(got.Records), got.Skipped)
	}
	if got.Records[1].Event != Away {
		t.Errorf("second record should be away")
	}
	if got.Records[4].Event != Lunch || got.Records[5].Event != AFK || got.Records[6].Event != Idle {
		t.Errorf("the last three records should be a lunch break, AFK and a quiet spell")
	}
	if c := got.Records[2].Context; c != "~/notes, drafts" {
		t.Errorf("context with a comma came back as %q", c)
	}
	if c := got.Records[3].Context; c != `say "hi"` {
		t.Errorf("context with quotes came back as %q", c)
	}
}

func TestParseSkipsBrokenRows(t *testing.T) {
	got, err := Parse(strings.NewReader(`timestamp,event,session,context
2026-11-10T09:05:00+11:00,focus,Minecraft,main
not a time,focus,Minecraft,main
2026-11-10T09:06:00+11:00,blink,Minecraft,main
2026-11-10T09:07:00+11:00,focus,,main
2026-11-10T09:08:00+11:00,focus,Mine`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Skipped != 4 {
		t.Fatalf("got %d records, %d skipped; want 1 and 4", len(got.Records), got.Skipped)
	}
}

func TestLoadAcrossMonths(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("2026-10.csv", "timestamp,event,session,context\n"+
		"2026-10-30T09:00:00+11:00,focus,A,x\n"+
		"2026-10-31T17:00:00+11:00,focus,A,y\n")
	write("2026-11.csv", "timestamp,event,session,context\n"+
		"2026-11-02T09:00:00+11:00,focus,B,z\n"+
		"2026-11-01T10:00:00+11:00,focus,B,w\n")

	day := func(m time.Month, d int) Date { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	got, err := Load(dir, day(10, 31), day(11, 1))
	if err != nil {
		t.Fatal(err)
	}
	var contexts []string
	for _, rec := range got.Records {
		contexts = append(contexts, rec.Context)
	}
	if strings.Join(contexts, " ") != "y w" {
		t.Errorf("got %v, want the two records on 31/10 and 01/11 in order", contexts)
	}
}

func TestDirPrefersTelemetryDir(t *testing.T) {
	t.Setenv("TELEMETRY_DIR", "/records")
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := Dir(); got != "/records" {
		t.Errorf("got %s", got)
	}
	t.Setenv("TELEMETRY_DIR", "")
	if got := Dir(); got != filepath.Join("/state", "telemetry") {
		t.Errorf("got %s", got)
	}
}
