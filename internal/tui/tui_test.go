package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
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

// click finds text on screen and clicks it, through the view's own hit testing
func click(t *testing.T, m Model, text string) Model {
	t.Helper()
	v := m.View()
	for y, line := range strings.Split(ansi.Strip(v.Content), "\n") {
		if x := strings.Index(line, text); x >= 0 {
			x = len([]rune(line[:x]))
			cmd := v.OnMouse(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			if cmd == nil {
				t.Fatalf("clicking %q did nothing", text)
			}
			next, cmd := m.Update(cmd())
			return settle(next.(Model), cmd)
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, ansi.Strip(v.Content))
	return m
}

func TestClickingATotalShowsEveryStretch(t *testing.T) {
	m := click(t, newModel(t), "release-1.0")
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"Roblox · release-1.0", "09:00 - 11:00", "2:00 over 1 day(s), in 1 stretch(es)"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	m = press(m, "esc")
	if strings.Contains(ansi.Strip(m.View().Content), "stretch(es)") {
		t.Error("esc should close the detail")
	}
}

func TestClickingTheLegendShowsTheSession(t *testing.T) {
	m := click(t, newModel(t), "Roblox    2:00")
	if len(m.details) != 1 || m.details[0] != (sheet.Selection{Session: "Roblox"}) {
		t.Errorf("the legend should open the whole session, got %v", m.details)
	}
}

func TestClickingOutsideClosesTheDetail(t *testing.T) {
	m := click(t, newModel(t), "release-1.0")
	m = click(t, m, "Summary")
	if len(m.details) != 0 {
		t.Error("a click outside the detail should close it")
	}
}

func TestDetailFollowsThePeriod(t *testing.T) {
	m := click(t, newModel(t), "release-1.0")
	m = press(m, "left")
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Week 45") || !strings.Contains(view, "Roblox · release-1.0") {
		t.Errorf("the detail should stay open on the week before:\n%s", view)
	}
}

func TestEnterOpensTheEntryInDays(t *testing.T) {
	m := press(newModel(t), "2", "k", "k", "enter")
	if len(m.details) != 1 || m.details[0].Title() != "Roblox · release-1.0" {
		t.Errorf("enter on Tuesday's entry should open it, got %v", m.details)
	}
}

func TestDetailFromTheCommandLine(t *testing.T) {
	m := newModel(t)
	m.cfg.Detail = &sheet.Selection{Session: "Roblox"}
	m = New(m.cfg)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = settle(next.(Model), m.load())
	if !strings.Contains(ansi.Strip(m.View().Content), "stretch(es)") {
		t.Error("a selection from the command line should open on start")
	}
}
