package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

var now = time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)

func newModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	body := "timestamp,event,session,context\n" +
		"2026-11-10T09:00:00Z,focus,Roblox,release-1.0\n" +
		"2026-11-10T11:00:00Z,focus,Roblox,release-1.0\n" +
		"2026-10-20T09:00:00Z,focus,Minecraft,main\n" +
		"2026-10-20T10:00:00Z,focus,Minecraft,main\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-11.csv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-10.csv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(Config{
		Dir:    dir,
		Period: period.Containing(period.Week, time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)),
		Rules:  sheet.DefaultRules,
		Now:    func() time.Time { return now },
	})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return settle(next.(Model), m.load())
}

// settle runs a load command and feeds its result back, as the program would
func settle(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	if msg, ok := cmd().(loadedMsg); ok {
		next, _ := m.Update(msg)
		return next.(Model)
	}
	return m
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		next, cmd := m.Update(msg)
		m = settle(next.(Model), cmd)
	}
	return m
}

func TestSummaryShowsTheWeek(t *testing.T) {
	m := newModel(t)
	view := m.View().Content
	for _, want := range []string{"Week 46", "Roblox", "release-1.0", "2:00"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary is missing %q:\n%s", want, view)
		}
	}
}

func TestDaysStartOnToday(t *testing.T) {
	m := press(newModel(t), "2")
	if !strings.Contains(m.View().Content, "Thursday 12/11/2026") {
		t.Errorf("the days view should open on today:\n%s", m.View().Content)
	}
	m = press(m, "k", "k")
	if !strings.Contains(m.View().Content, "Tuesday 10/11/2026") {
		t.Errorf("two up from Thursday is Tuesday:\n%s", m.View().Content)
	}
}

func TestPeriodsMoveButNotPastToday(t *testing.T) {
	m := press(newModel(t), "right")
	if !strings.Contains(m.View().Content, "Week 46") {
		t.Errorf("there is no week after the current one to move to")
	}
	m = press(m, "m", "left")
	if !strings.Contains(m.View().Content, "October 2026") || !strings.Contains(m.View().Content, "Minecraft") {
		t.Errorf("m then left should show October:\n%s", m.View().Content)
	}
	m = press(m, "c")
	if !strings.Contains(m.View().Content, "November 2026") {
		t.Errorf("c should come back to the current month")
	}
}

func TestTabsCycle(t *testing.T) {
	m := newModel(t)
	for _, want := range []view{daysView, timelineView, summaryView} {
		m = press(m, "tab")
		if m.view != want {
			t.Errorf("got view %d, want %d", m.view, want)
		}
	}
}

func TestStaleLoadsAreDropped(t *testing.T) {
	m := newModel(t)
	stale := loadedMsg{period: m.period.Shift(-5)}
	next, _ := m.Update(stale)
	if len(next.(Model).days) == 0 {
		t.Errorf("a load for a period since left should not replace the days shown")
	}
}

func TestQuit(t *testing.T) {
	_, cmd := newModel(t).Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q should send a quit message")
	}
}
