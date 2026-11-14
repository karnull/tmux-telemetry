package record

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// labels are the window names the plugin records by name, as it spells them
var labels = []string{"Bench", "Research", "Meeting", "Hardware"}

// Answer reads where a quiet spell went, the way the plugin's popup does: c to
// carry on with the work given, a for AFK, l for lunch, m for Meeting, e for
// External, a label in any case, or anything else as typed. Work goes to the
// session given, or Unnamed without one. An empty answer, or carrying on with
// no work given, leaves the spell as it was.
func Answer(at time.Time, session, context, where string) (Record, bool) {
	where = strings.TrimSpace(where)
	if session == "" {
		session = "Unnamed"
	}
	focus := func(context string) (Record, bool) {
		return Record{Time: at, Event: Focus, Session: session, Context: context}, true
	}
	switch strings.ToLower(where) {
	case "":
		return Record{}, false
	case "c", "continue":
		if context == "" {
			return Record{}, false
		}
		return focus(context)
	case "a", "afk":
		return Record{Time: at, Event: AFK}, true
	case "l", "lunch":
		return Record{Time: at, Event: Lunch}, true
	case "m":
		return focus("Meeting")
	case "e", "!meeting":
		return focus("External")
	}
	for _, label := range labels {
		if strings.EqualFold(where, label) {
			return focus(label)
		}
	}
	return focus(where)
}

var eventNames = map[Event]string{Focus: "focus", Away: "away", Lunch: "lunch", AFK: "afk", Idle: "idle"}

// Append adds a record to its month's file, as the plugin would have written it.
func Append(dir string, rec Record) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, rec.Time.Format("2006-01")+".csv")
	info, err := os.Stat(path)
	fresh := err != nil || info.Size() == 0

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	out := csv.NewWriter(file)
	if fresh {
		_ = out.Write([]string{"timestamp", "event", "session", "context"})
	}
	_ = out.Write([]string{rec.Time.Format(time.RFC3339), eventNames[rec.Event], rec.Session, rec.Context})
	out.Flush()
	if err := out.Error(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
