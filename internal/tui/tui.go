// Package tui browses timesheets in the terminal: a summary with a pie chart, each
// day's timetable, and a timeline of the whole period.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/record"
	"github.com/karnull/tmux-telemetry/internal/render"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Config is where the records are, and what to show first.
type Config struct {
	Dir    string
	Period period.Period
	Rules  sheet.Rules
	Now    func() time.Time
}

// refresh is how often a period still in progress is read again
const refresh = 30 * time.Second

type view int

const (
	summaryView view = iota
	daysView
	timelineView
	viewCount
)

var viewNames = [...]string{"Summary", "Days", "Timeline"}

type loadedMsg struct {
	period  period.Period
	days    []sheet.Day
	skipped int
	err     error
}

type tickMsg struct{}

// Model is the whole TUI's state.
type Model struct {
	cfg      Config
	period   period.Period
	days     []sheet.Day
	skipped  int
	err      error
	loaded   bool
	view     view
	selected int
	width    int
	height   int
	keys     keyMap
	help     help.Model
	totals   table.Model
	detail   viewport.Model
	theme    render.Theme
}

// New starts on the configured period.
func New(cfg Config) Model {
	totals := table.New(table.WithFocused(true))
	styles := table.DefaultStyles()
	styles.Selected = styles.Selected.Foreground(lipgloss.ANSIColor(13))
	totals.SetStyles(styles)
	m := Model{
		cfg:    cfg,
		period: cfg.Period,
		keys:   newKeyMap(),
		help:   help.New(),
		totals: totals,
		detail: viewport.New(),
		theme:  render.NewTheme(true, nil),
	}
	m.selected = m.todayIndex()
	return m
}

// todayIndex is today's place in the period, or the first day when it isn't in it.
func (m Model) todayIndex() int {
	if !m.period.Contains(m.today()) {
		return 0
	}
	return int(m.today().Sub(m.period.First).Hours() / 24)
}

// Run shows the TUI until it is quit.
func Run(ctx context.Context, cfg Config) error {
	_, err := tea.NewProgram(New(cfg), tea.WithContext(ctx)).Run()
	return err
}

func (m Model) load() tea.Cmd {
	cfg, p := m.cfg, m.period
	return func() tea.Msg {
		got, err := record.Load(cfg.Dir, p.First, p.Last)
		return loadedMsg{period: p, days: sheet.Build(got.Records, cfg.Rules), skipped: got.Skipped, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(refresh, func(time.Time) tea.Msg { return tickMsg{} })
}

// Init reads the first period and starts the refresh timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(), tick(), tea.RequestBackgroundColor)
}

func (m Model) today() record.Date { return record.DateOf(m.cfg.Now()) }

// Update handles keys, resizes and freshly read records.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.layout()
		return m, nil

	case tea.BackgroundColorMsg:
		m.help.Styles = help.DefaultStyles(msg.IsDark())
		return m, nil

	case loadedMsg:
		// a slow read for a period since left behind is dropped
		if msg.period != m.period {
			return m, nil
		}
		m.days, m.skipped, m.err, m.loaded = msg.days, msg.skipped, msg.err, true
		m.theme = render.NewTheme(true, sheet.Sessions(m.days))
		m.selected = min(m.selected, max(0, len(m.period.Dates())-1))
		m.layout()
		return m, nil

	case tickMsg:
		if m.period.Contains(m.today()) {
			return m, tea.Batch(m.load(), tick())
		}
		return m, tick()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.layout()
		return m, nil
	case key.Matches(msg, m.keys.NextView):
		m.view = (m.view + 1) % viewCount
	case key.Matches(msg, m.keys.PrevView):
		m.view = (m.view + viewCount - 1) % viewCount
	case key.Matches(msg, m.keys.Summary):
		m.view = summaryView
	case key.Matches(msg, m.keys.Days):
		m.view = daysView
	case key.Matches(msg, m.keys.Timeline):
		m.view = timelineView
	case key.Matches(msg, m.keys.Earlier):
		return m.goTo(m.period.Shift(-1))
	case key.Matches(msg, m.keys.Later):
		if next := m.period.Shift(1); !next.First.After(m.today()) {
			return m.goTo(next)
		}
		return m, nil
	case key.Matches(msg, m.keys.Today):
		unit := m.period.Unit
		if unit == period.Range {
			unit = period.Week
		}
		return m.goTo(period.Containing(unit, m.today()))
	case key.Matches(msg, m.keys.ByDay):
		return m.goTo(period.Containing(period.Day, m.anchor()))
	case key.Matches(msg, m.keys.ByWeek):
		return m.goTo(period.Containing(period.Week, m.anchor()))
	case key.Matches(msg, m.keys.ByMonth):
		return m.goTo(period.Containing(period.Month, m.anchor()))
	case key.Matches(msg, m.keys.Reload):
		return m, m.load()
	case m.view == daysView && key.Matches(msg, m.keys.Up):
		m.selected = max(0, m.selected-1)
		m.layout()
		return m, nil
	case m.view == daysView && key.Matches(msg, m.keys.Down):
		m.selected = min(len(m.period.Dates())-1, m.selected+1)
		m.layout()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.view {
	case summaryView:
		m.totals, cmd = m.totals.Update(msg)
	case daysView, timelineView:
		m.detail, cmd = m.detail.Update(msg)
	}
	m.layout()
	return m, cmd
}

// anchor is the day a change of unit keeps in view: the selected day, or today
// when today falls inside the period.
func (m Model) anchor() record.Date {
	if m.view == daysView {
		if dates := m.period.Dates(); m.selected < len(dates) {
			return dates[m.selected]
		}
	}
	if m.period.Contains(m.today()) {
		return m.today()
	}
	return m.period.First
}

func (m Model) goTo(p period.Period) (tea.Model, tea.Cmd) {
	m.period, m.loaded, m.days = p, false, nil
	m.selected = m.todayIndex()
	m.layout()
	return m, m.load()
}

func (m Model) day(date record.Date) (sheet.Day, bool) {
	for _, d := range m.days {
		if d.Date.Equal(date) {
			return d, true
		}
	}
	return sheet.Day{Date: date}, false
}

const (
	headerHeight = 2
	gap          = 3
)

func (m Model) bodyHeight() int {
	return max(1, m.height-headerHeight-lipgloss.Height(m.help.View(m.keys))-1)
}

func (m Model) leftWidth() int {
	return max(30, m.width*3/5)
}

// layout sizes and fills the table and viewport for the current view.
func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	body := m.bodyHeight()

	grand := sheet.Grand(m.days)
	left := m.leftWidth()
	timeWidth, shareWidth := 6, 18
	sessionWidth := max(8, (left-timeWidth-shareWidth-8)/3)
	m.totals.SetColumns([]table.Column{
		{Title: "Session", Width: sessionWidth},
		{Title: "Work", Width: max(8, left-sessionWidth-timeWidth-shareWidth-8)},
		{Title: "Time", Width: timeWidth},
		{Title: "Share", Width: shareWidth},
	})
	var rows []table.Row
	for _, t := range sheet.Totals(m.days) {
		share := 0.0
		if grand > 0 {
			share = t.Duration.Seconds() / grand.Seconds()
		}
		rows = append(rows, table.Row{t.Session, t.Context, sheet.Hours(t.Duration),
			fmt.Sprintf("%3.0f%% %s", 100*share, render.Bar(m.theme, t.Session, share, shareWidth-6))})
	}
	m.totals.SetRows(rows)
	m.totals.SetWidth(left)
	m.totals.SetHeight(body)

	switch m.view {
	case daysView:
		m.detail.SetWidth(max(10, m.width-m.dayListWidth()-gap))
		m.detail.SetHeight(body)
		m.detail.SetContent(m.dayDetail(m.width - m.dayListWidth() - gap))
	case timelineView:
		m.detail.SetWidth(m.width)
		m.detail.SetHeight(body)
		m.detail.SetContent(m.timelines())
	}
}

// View draws the tabs and period, the current view, and the keys.
func (m Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	if m.width == 0 {
		return v
	}

	var body string
	switch {
	case m.err != nil:
		body = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1)).Render("Could not read the records: " + m.err.Error())
	case !m.loaded:
		body = faint("Reading records…")
	case m.view == summaryView:
		body = m.summary()
	case m.view == daysView:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.dayList(), strings.Repeat(" ", gap), m.detail.View())
	case m.view == timelineView:
		body = m.detail.View()
	}
	body = lipgloss.NewStyle().Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)

	v.SetContent(lipgloss.JoinVertical(lipgloss.Left, m.header(), body, m.help.View(m.keys)))
	return v
}

func faint(s string) string { return lipgloss.NewStyle().Faint(true).Render(s) }

func (m Model) header() string {
	var tabs []string
	for i, name := range viewNames {
		style := lipgloss.NewStyle().Padding(0, 1)
		if view(i) == m.view {
			style = style.Reverse(true).Bold(true)
		} else {
			style = style.Faint(true)
		}
		tabs = append(tabs, style.Render(name))
	}
	left := strings.Join(tabs, " ")

	right := lipgloss.NewStyle().Bold(true).Render(m.period.Title())
	if m.loaded {
		right += "  " + faint(fmt.Sprintf("%s over %d day(s)", sheet.Hours(sheet.Grand(m.days)), len(m.days)))
	}
	if m.skipped > 0 {
		right += "  " + lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(3)).Render(fmt.Sprintf("%d unreadable", m.skipped))
	}
	space := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right + "\n"
}

func (m Model) summary() string {
	if len(m.days) == 0 {
		return faint("Nothing recorded for " + m.period.Title() + ".")
	}
	sessions := sheet.Sessions(m.days)
	legend := render.Legend(m.theme, sessions)
	right := m.width - m.leftWidth() - gap
	radius := min((m.bodyHeight() - lipgloss.Height(legend) - 1), right/2, 12)
	pie := ""
	if radius >= 3 {
		pie = render.Pie(m.theme, sessions, radius) + "\n\n"
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, m.totals.View(), strings.Repeat(" ", gap), pie+legend)
}

func (m Model) dayListWidth() int { return len("▸ Mon 09/11  10:00") + 2 }

func (m Model) dayList() string {
	var lines []string
	for i, date := range m.period.Dates() {
		d, ok := m.day(date)
		total := "    -"
		if ok {
			total = fmt.Sprintf("%5s", sheet.Hours(d.Total()))
		}
		line := fmt.Sprintf("  %s  %s", date.Format("Mon 02/01"), total)
		switch {
		case i == m.selected:
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(13)).Render("▸" + line[1:])
		case !ok:
			line = faint(line)
		}
		lines = append(lines, line)
	}
	// a month is longer than most terminals, so the list keeps the selection in view
	if body := m.bodyHeight(); len(lines) > body {
		start := min(max(0, m.selected-body/2), len(lines)-body)
		lines = lines[start : start+body]
	}
	return lipgloss.NewStyle().Width(m.dayListWidth()).Render(strings.Join(lines, "\n"))
}

func (m Model) dayDetail(width int) string {
	dates := m.period.Dates()
	if m.selected >= len(dates) {
		return ""
	}
	d, ok := m.day(dates[m.selected])
	title := lipgloss.NewStyle().Bold(true).Render(d.Date.Format("Monday 02/01/2006"))
	if !ok {
		return title + "\n\n" + faint("Nothing recorded.")
	}
	span := render.SpanOf([]sheet.Day{d}, m.cfg.Rules)
	barWidth := max(10, width-2)
	return strings.Join([]string{
		title + "  " + faint(sheet.Hours(d.Total())),
		"",
		" " + render.Axis(m.theme, span, barWidth),
		faint("│") + render.Timeline(m.theme, d, m.cfg.Rules, span, barWidth) + faint("│"),
		"",
		render.Timetable(m.theme, d),
	}, "\n")
}

func (m Model) timelines() string {
	if len(m.days) == 0 {
		return faint("Nothing recorded for " + m.period.Title() + ".")
	}
	return render.Timelines(m.theme, m.days, m.cfg.Rules, m.width) + "\n\n" + render.Legend(m.theme, sheet.Sessions(m.days))
}
