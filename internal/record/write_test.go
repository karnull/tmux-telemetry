package record

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnswer(t *testing.T) {
	at := time.Date(2026, 11, 12, 10, 0, 1, 0, time.UTC)
	tests := []struct {
		where   string
		event   Event
		session string
		context string
	}{
		{"a", AFK, "", ""},
		{"AFK", AFK, "", ""},
		{"lunch", Lunch, "", ""},
		{"m", Focus, "Minecraft", "Meeting"},
		{"e", Focus, "Minecraft", "External"},
		{"!Meeting", Focus, "Minecraft", "External"},
		{"research", Focus, "Minecraft", "Research"},
		{"fixing the printer", Focus, "Minecraft", "fixing the printer"},
	}
	for _, tt := range tests {
		got, ok := Answer(at, "Minecraft", tt.where)
		if !ok || got.Event != tt.event || got.Session != tt.session || got.Context != tt.context {
			t.Errorf("%q: got %+v", tt.where, got)
		}
	}
	if _, ok := Answer(at, "Minecraft", "  "); ok {
		t.Error("an empty answer should leave the spell as it was")
	}
	if got, _ := Answer(at, "", "m"); got.Session != "Unnamed" {
		t.Errorf("no session should go down as Unnamed, got %q", got.Session)
	}
}

func TestAppendReadsBack(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 11, 12, 10, 0, 1, 0, time.FixedZone("AEDT", 11*60*60))
	for _, rec := range []Record{
		{Time: at, Event: Idle},
		{Time: at.Add(time.Second), Event: Focus, Session: "Minecraft", Context: "notes, drafts"},
	} {
		if err := Append(dir, rec); err != nil {
			t.Fatal(err)
		}
	}
	body, _ := os.ReadFile(filepath.Join(dir, "2026-11.csv"))
	want := "timestamp,event,session,context\n" +
		"2026-11-12T10:00:01+11:00,idle,,\n" +
		"2026-11-12T10:00:02+11:00,focus,Minecraft,\"notes, drafts\"\n"
	if string(body) != want {
		t.Errorf("got\n%s", body)
	}
}
