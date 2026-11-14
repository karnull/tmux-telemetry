package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Thursday 12/11/2026
var now = time.Date(2026, 11, 12, 15, 0, 0, 0, time.FixedZone("AEDT", 11*60*60))

const records = `timestamp,event,session,context
2026-11-10T08:40:00+11:00,focus,Roblox,release-1.0
2026-11-10T11:58:00+11:00,focus,Roblox,release-1.0
2026-11-10T12:30:00+11:00,away,,
2026-11-10T13:32:00+11:00,focus,Minecraft,chore/get-started
2026-11-10T15:01:00+11:00,focus,Minecraft,chore/get-started
`

func execute(t *testing.T, vars map[string]string, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026-11.csv"), []byte(records), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := New(Env{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Now:     func() time.Time { return now },
		Getenv:  func(k string) string { return vars[k] },
		Environ: func() []string { return []string{"TERM=dumb"} },
	})
	cmd.SetArgs(append([]string{"--dir", dir}, args...))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

type report struct {
	Period struct{ First, Last string }
	Days   []struct {
		Entries []struct{ Start, End string }
	}
}

func jsonReport(t *testing.T, args ...string) report {
	t.Helper()
	out, _, err := execute(t, nil, append([]string{"-f", "json"}, args...)...)
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return r
}

func TestPeriodFlags(t *testing.T) {
	tests := []struct {
		args        string
		first, last string
	}{
		{"", "2026-11-02", "2026-11-08"},
		{"--week", "2026-11-02", "2026-11-08"},
		{"--week --previous", "2026-11-02", "2026-11-08"},
		{"-pw", "2026-11-02", "2026-11-08"},
		{"--week --current", "2026-11-09", "2026-11-15"},
		{"-cw", "2026-11-09", "2026-11-15"},
		{"-wc", "2026-11-09", "2026-11-15"},
		{"-c", "2026-11-09", "2026-11-15"},
		{"--day --current", "2026-11-12", "2026-11-12"},
		{"-cd", "2026-11-12", "2026-11-12"},
		{"--day", "2026-11-11", "2026-11-11"},
		{"-pd", "2026-11-11", "2026-11-11"},
		{"--month", "2026-10-01", "2026-10-31"},
		{"-pm", "2026-10-01", "2026-10-31"},
		{"-cm", "2026-11-01", "2026-11-30"},
		{"--day 11/04/2026", "2026-04-11", "2026-04-11"},
		{"--day=11/04/2026", "2026-04-11", "2026-04-11"},
		{"-d 11/04/2026", "2026-04-11", "2026-04-11"},
		{"-d=11/04/2026", "2026-04-11", "2026-04-11"},
		{"11/04/2026 --day", "2026-04-11", "2026-04-11"},
		{"--day monday", "2026-11-09", "2026-11-09"},
		{"--week 25,2026", "2026-06-15", "2026-06-21"},
		{"--week=2026-W25", "2026-06-15", "2026-06-21"},
		{"-w 25", "2026-06-15", "2026-06-21"},
		{"--week 11/04/2026", "2026-04-06", "2026-04-12"},
		{"--week 04/2026", "2026-01-19", "2026-01-25"},
		{"--month 04/2026", "2026-04-01", "2026-04-30"},
		{"--month April", "2026-04-01", "2026-04-30"},
		{"-m 4", "2026-04-01", "2026-04-30"},
		{"11/04/2026", "2026-04-11", "2026-04-11"},
		{"2026-W25", "2026-06-15", "2026-06-21"},
		{"04/2026", "2026-04-01", "2026-04-30"},
		{"-- yesterday", "2026-11-11", "2026-11-11"},
		{"-s 2026-11-01 -u 2026-11-05", "2026-11-01", "2026-11-05"},
		{"-s 01/11/2026 -u 05/11", "2026-11-01", "2026-11-05"},
		{"-s2026-11-10", "2026-11-10", "2026-11-12"},
		{"--since=monday", "2026-11-09", "2026-11-12"},
	}
	for _, tt := range tests {
		r := jsonReport(t, strings.Fields(tt.args)...)
		if r.Period.First != tt.first || r.Period.Last != tt.last {
			t.Errorf("%q: got %s to %s, want %s to %s", tt.args, r.Period.First, r.Period.Last, tt.first, tt.last)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range []string{
		"-cp",
		"-dw",
		"--day --week",
		"-c 11/04/2026",
		"--day 11/04/2026 --previous",
		"--day=11/04/2026 12/04/2026",
		"11/04/2026 12/04/2026",
		"25",
		"--day 31/04/2026",
		"--week 54,2026",
		"--month 13/2026",
		"-p -s 2026-11-01",
		"-s 2026-11-01 11/11/2026",
		"-u 2026-11-01",
		"-s 2026-11-10 -u 2026-11-01",
		"-s soon",
		"--lunch 12:00-13:00",
		"--day-start 09:00",
		"-f xml",
		"--colour sometimes",
		"-x",
	} {
		_, _, err := execute(t, nil, strings.Fields(args)...)
		if !IsUsage(err) {
			t.Errorf("%q: want a usage error, got %v", args, err)
		}
	}
}

func TestDayRunsFromItsFirstRecord(t *testing.T) {
	out, _, err := execute(t, nil, "-s", "2026-11-10", "-u", "2026-11-10", "-f", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026-11-10,08:40,12:30,Roblox") {
		t.Errorf("the day should start at its first record and run to the away:\n%s", out)
	}
}

func TestTextReport(t *testing.T) {
	out, _, err := execute(t, nil, "-s", "2026-11-10", "-u", "2026-11-10", "-t")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Tue 10/11/2026", "Roblox", "release-1.0", "5:19", "08:40 - 12:30", "13:32 - 15:01"} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("a dumb terminal should get no escape codes")
	}

	out, _, _ = execute(t, nil, "-cd")
	if !strings.Contains(out, "Nothing recorded") {
		t.Errorf("an empty day should say so:\n%s", out)
	}
}

func TestInteractiveNeedsATerminal(t *testing.T) {
	if _, _, err := execute(t, nil, "-i"); !IsUsage(err) {
		t.Errorf("-i without a terminal: got %v", err)
	}
}
