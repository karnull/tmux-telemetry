// Package tui browses timesheets in the terminal: a summary with a pie chart, each
// day's timetable, and a timeline of the whole period, with every stretch spent on
// any piece of work a click away.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/record"
	"github.com/karnull/tmux-telemetry/internal/render"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

// Config is where the records are, what to show first, and optionally the work
// whose detail opens straight away.
type Config struct {
	Dir    string
	Period period.Period
	Now    func() time.Time
	Detail *sheet.Selection
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

// clickMsg is a click on a layer, with where in the layer it landed.
type clickMsg struct {
	id   string
	x, y int
}

// Model is the whole TUI's state.
type Model struct {
	cfg     Config
	period  period.Period
	days    []sheet.Day
	skipped int
	err     error
	loaded  bool
	view    view
	width   int
	height  int
	keys    keyMap
	help    help.Model
	theme   render.Theme

	cursor   int // the totals row picked in the summary
	selected int // the day picked in the days view
	entry    int // the entry picked in that day
	scroll   int // how far the timeline view is scrolled

	// details open, the latest on top, so esc steps back through them
	details      []sheet.Selection
	detailScroll int

	// an unattended spell being said where it went, while the box is open
	fill *fill
}

// fill is the box asking where an unattended spell went
type fill struct {
	spell   string
	at      time.Time
	session string
	input   textinput.Model
}

// New starts on the configured period.
func New(cfg Config) Model {
	m := Model{
		cfg:    cfg,
		period: cfg.Period,
		keys:   newKeyMap(),
		help:   help.New(),
		theme:  render.NewTheme(true, nil),
	}
	if cfg.Detail != nil {
		m.details = []sheet.Selection{*cfg.Detail}
	}
	m.selected = m.todayIndex()
	return m
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
		return loadedMsg{period: p, days: sheet.Build(got.Records), skipped: got.Skipped, err: err}
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

// todayIndex is today's place in the period, or the first day when it isn't in it.
func (m Model) todayIndex() int {
	if !m.period.Contains(m.today()) {
		return 0
	}
	return int(m.today().Sub(m.period.First).Hours() / 24)
}

func (m Model) totals() []sheet.Total { return sheet.Totals(m.days) }

func (m Model) day(date record.Date) (sheet.Day, bool) {
	for _, d := range m.days {
		if d.Date.Equal(date) {
			return d, true
		}
	}
	return sheet.Day{Date: date}, false
}

func (m Model) selectedDay() (sheet.Day, bool) {
	dates := m.period.Dates()
	if m.selected >= len(dates) {
		return sheet.Day{}, false
	}
	return m.day(dates[m.selected])
}

// Update handles keys, clicks, resizes and freshly read records.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
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
		m.cursor = min(m.cursor, max(0, len(m.totals())-1))
		m.selected = min(m.selected, max(0, len(m.period.Dates())-1))
		m.clampEntry()
		return m, nil

	case tickMsg:
		if m.period.Contains(m.today()) {
			return m, tea.Batch(m.load(), tick())
		}
		return m, tick()

	case clickMsg:
		return m.click(msg)

	case tea.MouseWheelMsg:
		step := 3
		if msg.Button == tea.MouseWheelUp {
			step = -3
		}
		return m.scrollBy(step), nil

	case tea.KeyPressMsg:
		if m.fill != nil {
			return m.handleFill(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

// startFill opens the box for the picked entry, when it is an unattended spell.
// The answer goes to the session worked on just before it.
func (m Model) startFill() (Model, tea.Cmd) {
	d, ok := m.selectedDay()
	if m.view != daysView || !ok || m.entry >= len(d.Entries) || d.Entries[m.entry].Break != sheet.UnattendedBreak {
		return m, nil
	}
	e := d.Entries[m.entry]
	session := ""
	for i := m.entry - 1; i >= 0; i-- {
		if d.Entries[i].Counted() {
			session = d.Entries[i].Session
			break
		}
	}
	y, mo, dd := d.Date.Date()
	input := textinput.New()
	input.Placeholder = "a for AFK, l lunch, m Meeting, e External, or what it was"
	input.SetWidth(56)
	f := &fill{
		spell:   sheet.Clock(e.Start) + " - " + sheet.Clock(e.End),
		at:      time.Date(y, mo, dd, 0, 0, 0, 0, time.Local).Add(e.Start + time.Second),
		session: session,
		input:   input,
	}
	m.fill = f
	return m, f.input.Focus()
}

func (m Model) handleFill(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fill = nil
		return m, nil
	case "enter":
		rec, ok := record.Answer(m.fill.at, m.fill.session, m.fill.input.Value())
		m.fill = nil
		if !ok {
			return m, nil
		}
		if err := record.Append(m.cfg.Dir, rec); err != nil {
			m.err = err
			return m, nil
		}
		return m, m.load()
	}
	var cmd tea.Cmd
	m.fill.input, cmd = m.fill.input.Update(msg)
	return m, cmd
}

func (m Model) scrollBy(step int) Model {
	switch {
	case len(m.details) > 0:
		m.detailScroll = max(0, m.detailScroll+step)
	case m.view == summaryView:
		m.cursor = clamp(m.cursor+step/3, 0, len(m.totals())-1)
	case m.view == daysView:
		m.entry += step / 3
		m.clampEntry()
	case m.view == timelineView:
		m.scroll = max(0, m.scroll+step)
	}
	return m
}

func clamp(v, low, high int) int { return max(low, min(v, high)) }

func (m *Model) clampEntry() {
	d, _ := m.selectedDay()
	m.entry = clamp(m.entry, 0, len(d.Entries)-1)
}

func (m Model) openDetail(sel sheet.Selection) Model {
	m.details = append(m.details, sel)
	m.detailScroll = 0
	return m
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Back):
		if len(m.details) > 0 {
			m.details = m.details[:len(m.details)-1]
			m.detailScroll = 0
		}
		return m, nil
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
	}

	// with a detail open, the arrows scroll it rather than what is behind it
	if len(m.details) > 0 {
		switch {
		case key.Matches(msg, m.keys.Up):
			return m.scrollBy(-1), nil
		case key.Matches(msg, m.keys.Down):
			return m.scrollBy(1), nil
		}
		return m, nil
	}

	switch {
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
	case key.Matches(msg, m.keys.Up):
		return m.move(-1), nil
	case key.Matches(msg, m.keys.Down):
		return m.move(1), nil
	case m.view == daysView && key.Matches(msg, m.keys.EntryUp):
		m.entry--
		m.clampEntry()
	case m.view == daysView && key.Matches(msg, m.keys.EntryDown):
		m.entry++
		m.clampEntry()
	case key.Matches(msg, m.keys.Open):
		if w, ok := m.picked(); ok {
			return m.openDetail(sheet.Select(w)), nil
		}
	case key.Matches(msg, m.keys.Fill):
		return m.startFill()
	case key.Matches(msg, m.keys.Session):
		if w, ok := m.picked(); ok {
			sel := sheet.Select(w)
			sel.Context = ""
			return m.openDetail(sel), nil
		}
	}
	return m, nil
}

// move steps the cursor of the current view
func (m Model) move(step int) Model {
	switch m.view {
	case summaryView:
		m.cursor = clamp(m.cursor+step, 0, len(m.totals())-1)
	case daysView:
		m.selected = clamp(m.selected+step, 0, len(m.period.Dates())-1)
		m.entry = 0
	case timelineView:
		m.scroll = max(0, m.scroll+step)
	}
	return m
}

// picked is the work under the cursor: the totals row, or the day's entry
func (m Model) picked() (sheet.Work, bool) {
	switch m.view {
	case summaryView:
		if t := m.totals(); m.cursor < len(t) {
			return t[m.cursor].Work, true
		}
	case daysView:
		if d, ok := m.selectedDay(); ok && m.entry < len(d.Entries) {
			return d.Entries[m.entry].Work, true
		}
	}
	return sheet.Work{}, false
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

// goTo changes period, keeping any detail open so the same work can be followed
// from one period to the next.
func (m Model) goTo(p period.Period) (tea.Model, tea.Cmd) {
	m.period, m.loaded, m.days = p, false, nil
	m.selected, m.entry, m.cursor, m.scroll, m.detailScroll = m.todayIndex(), 0, 0, 0, 0
	return m, m.load()
}

func (m Model) click(msg clickMsg) (tea.Model, tea.Cmd) {
	kind, arg, _ := strings.Cut(msg.id, ":")
	n, _ := strconv.Atoi(arg)

	if len(m.details) > 0 {
		switch kind {
		case "detail":
			if lines := m.detailLines(); n < len(lines) && lines[n].Select != nil {
				return m.openDetail(*lines[n].Select), nil
			}
		case "detailbox":
		default:
			// a click outside the detail closes it
			m.details, m.detailScroll = nil, 0
		}
		return m, nil
	}

	switch kind {
	case "tab":
		m.view = view(n)
	case "total":
		if t := m.totals(); n < len(t) {
			m.cursor = n
			return m.openDetail(sheet.Select(t[n].Work)), nil
		}
	case "legend":
		return m.openDetail(sheet.Selection{Session: arg}), nil
	case "day":
		m.selected, m.entry = n, 0
	case "entry":
		if d, ok := m.selectedDay(); ok && n < len(d.Entries) {
			m.entry = n
			return m.openDetail(sheet.Select(d.Entries[n].Work)), nil
		}
	case "dayline":
		if d, ok := m.selectedDay(); ok {
			if e, ok := render.At(d, render.SpanOf([]sheet.Day{d}), m.dayBarWidth(), msg.x-1); ok {
				return m.openDetail(sheet.Select(e.Work)), nil
			}
		}
	case "timeline":
		if n < len(m.days) {
			if e, ok := render.At(m.days[n], render.SpanOf(m.days), m.timelineBarWidth(), msg.x-timelineLabel-1); ok {
				return m.openDetail(sheet.Select(e.Work)), nil
			}
		}
	}
	return m, nil
}

//------------------------------------------------------------------------------
// layout

const (
	headerHeight  = 2
	gap           = 3
	timelineLabel = len("Mon 09/11 ")
)

func (m Model) helpView() string { return m.help.View(m.keys) }

func (m Model) bodyHeight() int {
	return max(1, m.height-headerHeight-lipgloss.Height(m.helpView())-1)
}

func (m Model) leftWidth() int { return max(30, m.width*3/5) }

func (m Model) dayListWidth() int { return len("▸ Mon 09/11  10:00") + 2 }

func (m Model) dayPaneWidth() int { return max(20, m.width-m.dayListWidth()-gap) }

func (m Model) dayBarWidth() int { return max(10, m.dayPaneWidth()-2) }

func (m Model) timelineBarWidth() int { return max(10, m.width-timelineLabel-len(" 10:00")-2) }

func faint(s string) string { return lipgloss.NewStyle().Faint(true).Render(s) }

func bold(s string) string { return lipgloss.NewStyle().Bold(true).Render(s) }

func fit(s string, width int) string { return lipgloss.NewStyle().MaxWidth(width).Render(s) }

// layers sharing a z-index may be drawn in any order, so each tier has its own
const (
	zBase = iota
	zContent
	zDetail
	zDetailLines
)

func layer(s string, x, y int, id string) *lipgloss.Layer {
	l := lipgloss.NewLayer(s).X(x).Y(y).Z(zContent)
	if id != "" {
		l = l.ID(id)
	}
	return l
}

// lines places each line of s as its own layer, so each can be hit separately
func lines(s string, x, y int, id func(i int) string) []*lipgloss.Layer {
	var out []*lipgloss.Layer
	for i, line := range strings.Split(s, "\n") {
		name := ""
		if id != nil {
			name = id(i)
		}
		out = append(out, layer(line, x, y+i, name))
	}
	return out
}

// View draws the tabs and period, the current view, any open detail on top, and
// the keys. Every part that can be clicked is a layer with an ID, and the same
// layers answer the clicks, so what shows and what is hit always agree.
func (m Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if m.width == 0 {
		return v
	}

	blank := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", m.width)+"\n", m.height), "\n")
	layers := []*lipgloss.Layer{layer(blank, 0, 0, "").Z(zBase)}
	layers = append(layers, m.headerLayers()...)

	switch {
	case m.err != nil:
		layers = append(layers, layer(lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1)).Render("Could not read the records: "+m.err.Error()), 0, headerHeight, ""))
	case !m.loaded:
		layers = append(layers, layer(faint("Reading records…"), 0, headerHeight, ""))
	case m.view == summaryView:
		layers = append(layers, m.summaryLayers()...)
	case m.view == daysView:
		layers = append(layers, m.daysLayers()...)
	case m.view == timelineView:
		layers = append(layers, m.timelineLayers()...)
	}
	if len(m.details) > 0 && m.loaded {
		layers = append(layers, m.detailLayers()...)
	}
	if m.fill != nil {
		layers = append(layers, m.fillLayer())
	}
	layers = append(layers, layer(m.helpView(), 0, m.height-lipgloss.Height(m.helpView()), ""))

	comp := lipgloss.NewCompositor(layers...)
	v.SetContent(comp.Render())
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		click, ok := msg.(tea.MouseClickMsg)
		if !ok || click.Button != tea.MouseLeft {
			return nil
		}
		hit := comp.Hit(click.X, click.Y)
		at := hit.Bounds().Min
		return func() tea.Msg { return clickMsg{id: hit.ID(), x: click.X - at.X, y: click.Y - at.Y} }
	}
	return v
}

func (m Model) headerLayers() []*lipgloss.Layer {
	var out []*lipgloss.Layer
	x := 0
	for i, name := range viewNames {
		style := lipgloss.NewStyle().Padding(0, 1)
		if view(i) == m.view {
			style = style.Reverse(true).Bold(true)
		} else {
			style = style.Faint(true)
		}
		tab := style.Render(name)
		out = append(out, layer(tab, x, 0, fmt.Sprintf("tab:%d", i)))
		x += lipgloss.Width(tab) + 1
	}

	right := bold(m.period.Title())
	if m.loaded {
		right += "  " + faint(render.Summary(m.days))
	}
	if m.skipped > 0 {
		right += "  " + lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(3)).Render(fmt.Sprintf("%d unreadable", m.skipped))
	}
	right = fit(right, max(1, m.width-x-1))
	return append(out, layer(right, m.width-lipgloss.Width(right), 0, ""))
}

func nothing(p period.Period) string { return faint("Nothing recorded for " + p.Title() + ".") }

// window is the part of a list of n rows that fits in height, keeping cursor in it
func window(n, cursor, height int) (start, end int) {
	if n <= height {
		return 0, n
	}
	start = clamp(cursor-height/2, 0, n-height)
	return start, start + height
}

func (m Model) summaryLayers() []*lipgloss.Layer {
	if len(m.days) == 0 {
		return []*lipgloss.Layer{layer(nothing(m.period), 0, headerHeight, "")}
	}
	var out []*lipgloss.Layer
	left := m.leftWidth()
	totals := m.totals()
	grand := sheet.Grand(m.days)

	const timeWidth, barWidth = 6, 10
	sessionWidth := clamp((left-timeWidth-barWidth-12)/3, 8, 20)
	workWidth := max(8, left-sessionWidth-timeWidth-barWidth-12)
	row := func(pointer, session, work, hours, share string) string {
		return pointer + " " + lipgloss.NewStyle().Width(sessionWidth).MaxWidth(sessionWidth).Render(session) + " " +
			lipgloss.NewStyle().Width(workWidth).MaxWidth(workWidth).Render(work) + " " +
			lipgloss.NewStyle().Width(timeWidth).Align(lipgloss.Right).Render(hours) + "  " + share
	}
	out = append(out, layer(bold(row(" ", "Session", "Work", "Time", "Share")), 0, headerHeight, ""))

	start, end := window(len(totals), m.cursor, m.bodyHeight()-1)
	for i := start; i < end; i++ {
		t := totals[i]
		share := render.NotCounted
		if t.Counted() && grand > 0 {
			s := t.Duration.Seconds() / grand.Seconds()
			bar := lipgloss.NewStyle().Width(barWidth).Render(render.Bar(m.theme, t.Session, s, barWidth))
			share = fmt.Sprintf("%s %3.0f%%", bar, 100*s)
		}
		pointer, name := " ", m.theme.Paint(t.Session, t.Session)
		if !t.Counted() {
			name = faint(t.Session)
		}
		text := row(pointer, m.theme.Mark(t.Session)+" "+name, t.Context, sheet.Hours(t.Duration), share)
		if i == m.cursor {
			text = row(lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(13)).Render("▸"),
				m.theme.Mark(t.Session)+" "+bold(name), bold(t.Context), bold(sheet.Hours(t.Duration)), share)
		}
		out = append(out, layer(text, 0, headerHeight+1+i-start, fmt.Sprintf("total:%d", i)))
	}

	// the pie and its legend, each legend row opening its whole session
	sessions := sheet.Sessions(m.days)
	right := m.width - left - gap
	legend := render.Legend(m.theme, sessions)
	radius := min(m.bodyHeight()-lipgloss.Height(legend)-1, right/2, 12)
	y := headerHeight
	if radius >= 3 {
		pie := render.Pie(m.theme, sessions, radius)
		out = append(out, layer(pie, left+gap, y, ""))
		y += lipgloss.Height(pie) + 1
	}
	out = append(out, lines(legend, left+gap, y, func(i int) string { return "legend:" + sessions[i].Session })...)
	return out
}

func (m Model) daysLayers() []*lipgloss.Layer {
	var out []*lipgloss.Layer
	dates := m.period.Dates()
	start, end := window(len(dates), m.selected, m.bodyHeight())
	for i := start; i < end; i++ {
		d, ok := m.day(dates[i])
		total := "    -"
		if ok {
			total = fmt.Sprintf("%5s", sheet.Hours(d.Total()))
		}
		line := fmt.Sprintf("  %s  %s", dates[i].Format("Mon 02/01"), total)
		switch {
		case i == m.selected:
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(13)).Render("▸" + line[1:])
		case !ok:
			line = faint(line)
		}
		out = append(out, layer(line, 0, headerHeight+i-start, fmt.Sprintf("day:%d", i)))
	}

	x := m.dayListWidth() + gap
	d, ok := m.selectedDay()
	title := bold(d.Date.Format("Monday 02/01/2006"))
	if !ok {
		return append(out, layer(title+"\n\n"+faint("Nothing recorded."), x, headerHeight, ""))
	}
	span := render.SpanOf([]sheet.Day{d})
	y := headerHeight
	out = append(out,
		layer(title+"  "+faint(render.Summary([]sheet.Day{d})), x, y, ""),
		layer(" "+render.Axis(m.theme, span, m.dayBarWidth()), x, y+2, ""),
		layer(faint("│")+render.Timeline(m.theme, d, span, m.dayBarWidth())+faint("│"), x, y+3, "dayline"),
	)

	// the timetable's own rows follow its top border, header and rule
	table := strings.Split(render.Timetable(m.theme, d), "\n")
	const above = 3
	room := max(1, m.bodyHeight()-5)
	first, last := 0, len(table)
	if len(table) > room {
		first = clamp(m.entry+above-room/2, 0, len(table)-room)
		last = first + room
	}
	for i := first; i < last; i++ {
		id := ""
		if entry := i - above; entry >= 0 && entry < len(d.Entries) {
			id = fmt.Sprintf("entry:%d", entry)
			if entry == m.entry {
				out = append(out, layer(lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(13)).Render("▸"), x-2, y+5+i-first, ""))
			}
		}
		out = append(out, layer(fit(table[i], m.dayPaneWidth()), x, y+5+i-first, id))
	}
	return out
}

func (m Model) timelineLayers() []*lipgloss.Layer {
	if len(m.days) == 0 {
		return []*lipgloss.Layer{layer(nothing(m.period), 0, headerHeight, "")}
	}
	rows := strings.Split(render.Timelines(m.theme, m.days, m.width), "\n")
	sessions := sheet.Sessions(m.days)
	legend := strings.Split(render.Legend(m.theme, sessions), "\n")

	type line struct {
		text, id string
	}
	all := []line{{text: rows[0]}}
	for i, r := range rows[1:] {
		all = append(all, line{r, fmt.Sprintf("timeline:%d", i)})
	}
	all = append(all, line{})
	for i, l := range legend {
		all = append(all, line{l, "legend:" + sessions[i].Session})
	}

	start := min(m.scroll, max(0, len(all)-m.bodyHeight()))
	var out []*lipgloss.Layer
	for i := start; i < min(len(all), start+m.bodyHeight()); i++ {
		out = append(out, layer(all[i].text, 0, headerHeight+i-start, all[i].id))
	}
	return out
}

func (m Model) fillLayer() *lipgloss.Layer {
	where := "it"
	if m.fill.session != "" {
		where = m.fill.session
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.ANSIColor(13)).
		Padding(1, 2).
		Render(strings.Join([]string{
			bold("Where did " + m.fill.spell + " go?"),
			faint("Work goes to " + where + " · enter to keep it, esc to leave it UNATTENDED"),
			"",
			m.fill.input.View(),
		}, "\n"))
	return layer(box, max(0, (m.width-lipgloss.Width(box))/2), headerHeight+2, "fillbox").Z(zDetailLines + 1)
}

func (m Model) detailWidth() int { return clamp(m.width-6, 30, 110) }

func (m Model) detailLines() []render.DetailLine {
	if len(m.details) == 0 {
		return nil
	}
	return render.Detail(m.theme, m.days, m.details[len(m.details)-1], m.detailWidth()-4)
}

// detailLayers is the open detail as a bordered box over the view, its lines
// each a layer above it, so a click on one opens that narrower work
func (m Model) detailLayers() []*lipgloss.Layer {
	all := m.detailLines()
	width := m.detailWidth()
	height := min(m.bodyHeight(), len(all)+4)
	room := height - 4
	start := min(m.detailScroll, max(0, len(all)-room))

	trail := "esc back"
	if len(m.details) > 1 {
		trail = fmt.Sprintf("esc back to %s", m.details[len(m.details)-2].Title())
	}
	if len(all) > room {
		trail += fmt.Sprintf("  ·  %d-%d of %d", start+1, min(len(all), start+room), len(all))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.ANSIColor(13)).
		Width(width).Height(height).
		Render(strings.Repeat("\n", room) + "\n " + faint(trail))
	x := (m.width - lipgloss.Width(box)) / 2
	y := headerHeight
	out := []*lipgloss.Layer{layer(box, x, y, "detailbox").Z(zDetail)}
	for i := start; i < min(len(all), start+room); i++ {
		id := ""
		if all[i].Select != nil {
			id = fmt.Sprintf("detail:%d", i)
		}
		out = append(out, layer(fit(all[i].Text, width-4), x+2, y+1+i-start, id).Z(zDetailLines))
	}
	return out
}
