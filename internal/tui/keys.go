package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	NextView  key.Binding
	PrevView  key.Binding
	Summary   key.Binding
	Days      key.Binding
	Timeline  key.Binding
	Earlier   key.Binding
	Later     key.Binding
	Today     key.Binding
	ByDay     key.Binding
	ByWeek    key.Binding
	ByMonth   key.Binding
	Up        key.Binding
	Down      key.Binding
	EntryUp   key.Binding
	EntryDown key.Binding
	Open      key.Binding
	Session   key.Binding
	Back      key.Binding
	Reload    key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		NextView:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
		PrevView:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous view")),
		Summary:   key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "summary")),
		Days:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "days")),
		Timeline:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "timeline")),
		Earlier:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "earlier")),
		Later:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "later")),
		Today:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "current")),
		ByDay:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "by day")),
		ByWeek:    key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "by week")),
		ByMonth:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "by month")),
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		EntryUp:   key.NewBinding(key.WithKeys("K", "shift+up"), key.WithHelp("K", "entry up")),
		EntryDown: key.NewBinding(key.WithKeys("J", "shift+down"), key.WithHelp("J", "entry down")),
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter/click", "every stretch of it")),
		Session:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "its whole session")),
		Back:      key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Reload:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextView, k.Earlier, k.Later, k.Open, k.Back, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextView, k.PrevView, k.Summary, k.Days, k.Timeline},
		{k.Earlier, k.Later, k.Today, k.ByDay, k.ByWeek, k.ByMonth},
		{k.Up, k.Down, k.EntryUp, k.EntryDown},
		{k.Open, k.Session, k.Back, k.Reload, k.Help, k.Quit},
	}
}
