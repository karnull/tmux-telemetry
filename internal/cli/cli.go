// Package cli is the timesheet command: its flags, and how they pick a period.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/karnull/tmux-telemetry/internal/export"
	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/record"
	"github.com/karnull/tmux-telemetry/internal/render"
	"github.com/karnull/tmux-telemetry/internal/sheet"
	"github.com/karnull/tmux-telemetry/internal/tui"
)

// UsageError is a mistake in how timesheet was called, which exits with status 2.
type UsageError struct{ error }

func usagef(format string, args ...any) error {
	return UsageError{fmt.Errorf(format, args...)}
}

// unitFlag is --day, --week or --month: on its own like a switch, or with the
// period itself, as --day=11/04/2026 or --day 11/04/2026.
type unitFlag struct {
	on    bool
	value string
}

func (f *unitFlag) String() string { return "" }

// Type reads as a switch, so help shows it without a value placeholder.
func (f *unitFlag) Type() string { return "bool" }

func (f *unitFlag) Set(s string) error {
	switch s {
	case "true":
		f.on = true
	case "false":
		f.on, f.value = false, ""
	default:
		f.on, f.value = true, s
	}
	return nil
}

func (f *unitFlag) IsBoolFlag() bool { return true }

type options struct {
	previous, current bool
	day, week, month  unitFlag
	when              string

	since, until string

	dir string

	session, work string

	format      string
	color       string
	timetable   bool
	interactive bool
}

// Env is what the command reads from outside its flags, so tests can stand in for it.
type Env struct {
	Stdout, Stderr io.Writer
	Now            func() time.Time
	Getenv         func(string) string
	Environ        func() []string
}

// System is the real environment.
func System() Env {
	return Env{Stdout: os.Stdout, Stderr: os.Stderr, Now: time.Now, Getenv: os.Getenv, Environ: os.Environ}
}

// New builds the root command.
func New(env Env) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "timesheet [when]",
		Short: "Breaks tmux-telemetry records down into a timesheet",
		Long: `Breaks tmux-telemetry records down into a timesheet: the time spent on each
session and branch, each session's share as a pie chart, and a timeline of
every day. Without a period, it reports the previous week.

--day, --week and --month take a period of their own, or pair with --current or
--previous on their own. A period given without them is read by its form.

Days go day first: 11/04/2026 is the 11th of April. Weeks are ISO weeks, Monday
to Sunday. Short options group the POSIX way, so -cd is -c -d: today.`,
		Example: `# the previous week
timesheet

# today so far, and yesterday with its timetable
timesheet --day --current
timesheet -pd -t

# a day, a week and a month of their own
timesheet --day 11/04/2026
timesheet --week 25,2026
timesheet --month "April 2026"

# the same, read by their form
timesheet 11/04/2026
timesheet 2026-W25
timesheet 04/2026

# this week so far, and the previous month
timesheet -cw
timesheet -pm

# every stretch spent on one branch last week, or on a whole session this month
timesheet --session Minecraft --work chore/get-started
timesheet --session Minecraft -cm

# a range of days
timesheet -s 2026-11-01 -u 2026-11-15

# browse in the terminal
timesheet -i

# the previous month for a spreadsheet
timesheet -pm -f csv > november.csv`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usagef("timesheet takes one period at most, but was given %q", strings.Join(args, " "))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.when = args[0]
			}
			return run(cmd, env, opts)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return UsageError{err} })
	// --help says it all, so the help command still works but stays out of the list
	cmd.SetHelpCommand(&cobra.Command{
		Use:    "help",
		Hidden: true,
		Run:    func(c *cobra.Command, _ []string) { _ = c.Root().Help() },
	})

	f := cmd.Flags()
	f.SortFlags = false
	f.BoolVarP(&opts.current, "current", "c", false, "the day, week or month in progress")
	f.BoolVarP(&opts.previous, "previous", "p", false, "the day, week or month before it (the default)")
	unit := func(v *unitFlag, name, short, usage string) {
		f.VarPF(v, name, short, usage).NoOptDefVal = "true"
	}
	unit(&opts.day, "day", "d", "a day, such as "+period.DayForms)
	unit(&opts.week, "week", "w", "a week, such as "+period.WeekForms+" (the default)")
	unit(&opts.month, "month", "m", "a month, such as "+period.MonthForms)
	f.StringVarP(&opts.since, "since", "s", "", "first day of a range, in any form --day takes")
	f.StringVarP(&opts.until, "until", "u", "", "last day of a range, in any form --day takes (default today)")
	f.StringVar(&opts.dir, "dir", "", "where the records are (default $TELEMETRY_DIR, or $XDG_STATE_HOME/telemetry)")
	f.StringVarP(&opts.format, "format", "f", "text", "output as text, json or csv")
	f.StringVar(&opts.color, "colour", "auto", "colour the output: auto, always or never ($NO_COLOR is honoured)")
	f.StringVar(&opts.color, "color", "auto", "same as --colour")
	_ = f.MarkHidden("color")
	f.StringVar(&opts.session, "session", "", "only this session, or afk or lunch, with every stretch spent on it")
	f.StringVar(&opts.work, "work", "", "only this branch, folder or label, with every stretch spent on it")
	f.BoolVarP(&opts.timetable, "timetable", "t", false, "add each day's timetable to the text report")
	f.BoolVarP(&opts.interactive, "interactive", "i", false, "browse the report in the terminal")

	return cmd
}

type unitChoice struct {
	flag unitFlag
	unit period.Unit
	name string
}

// resolve picks the period the flags and operand ask for, as of today.
func (o options) resolve(today record.Date) (period.Period, error) {
	var picked []unitChoice
	for _, u := range []unitChoice{{o.day, period.Day, "--day"}, {o.week, period.Week, "--week"}, {o.month, period.Month, "--month"}} {
		if u.flag.on {
			picked = append(picked, u)
		}
	}
	if len(picked) > 1 {
		return period.Period{}, usagef("pick one of --day, --week or --month")
	}
	if o.current && o.previous {
		return period.Period{}, usagef("pick one of --current or --previous")
	}

	when := o.when
	if len(picked) == 1 && picked[0].flag.value != "" {
		if when != "" {
			return period.Period{}, usagef("%s was given %q, so %q has nowhere to go", picked[0].name, picked[0].flag.value, when)
		}
		when = picked[0].flag.value
	}

	if o.since != "" || o.until != "" {
		if len(picked) > 0 || o.current || o.previous || when != "" {
			return period.Period{}, usagef("--since and --until take the place of a period, so they can't be used with one")
		}
		return o.resolveRange(today)
	}

	if when != "" {
		if o.current || o.previous {
			return period.Period{}, usagef("%q is a period of its own, so it can't be used with --current or --previous", when)
		}
		p, err := parseWhen(when, picked, today)
		if err != nil {
			return p, UsageError{err}
		}
		return p, nil
	}

	unit, direction := period.Week, -1
	if len(picked) == 1 {
		unit = picked[0].unit
	}
	if o.current {
		direction = 0
	}
	return period.Containing(unit, today).Shift(direction), nil
}

// parseWhen reads a period by the unit picked, or by its form when none was.
func parseWhen(when string, picked []unitChoice, today record.Date) (period.Period, error) {
	if len(picked) == 0 {
		return period.Guess(when, today)
	}
	switch picked[0].unit {
	case period.Day:
		day, err := period.ParseDay(when, today)
		return period.Containing(period.Day, day), err
	case period.Month:
		return period.ParseMonth(when, today)
	default:
		return period.ParseWeek(when, today)
	}
}

func (o options) resolveRange(today record.Date) (period.Period, error) {
	if o.since == "" {
		return period.Period{}, usagef("--until needs --since")
	}
	first, err := period.ParseDay(o.since, today)
	if err != nil {
		return period.Period{}, usagef("--since: %w", err)
	}
	last := today
	if o.until != "" {
		if last, err = period.ParseDay(o.until, today); err != nil {
			return period.Period{}, usagef("--until: %w", err)
		}
	}
	p, err := period.Between(first, last)
	if err != nil {
		return p, UsageError{err}
	}
	return p, nil
}

func (o options) profile(env Env) (colorprofile.Profile, error) {
	detected := colorprofile.Detect(env.Stdout, env.Environ())
	switch o.color {
	case "auto":
		return detected, nil
	case "always":
		return max(detected, colorprofile.ANSI), nil
	case "never":
		return colorprofile.ASCII, nil
	}
	return 0, usagef("--colour takes auto, always or never, not %q", o.color)
}

func width(w io.Writer, env Env) int {
	if f, ok := w.(*os.File); ok {
		if cols, _, err := term.GetSize(f.Fd()); err == nil && cols > 0 {
			return cols
		}
	}
	var cols int
	if _, err := fmt.Sscan(env.Getenv("COLUMNS"), &cols); err == nil && cols > 0 {
		return cols
	}
	return 80
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func run(cmd *cobra.Command, env Env, opts options) error {
	p, err := opts.resolve(record.DateOf(env.Now()))
	if err != nil {
		return err
	}
	profile, err := opts.profile(env)
	if err != nil {
		return err
	}
	dir := opts.dir
	if dir == "" {
		dir = record.Dir()
	}

	if opts.interactive {
		if opts.format != "text" {
			return usagef("--interactive shows the report itself, so it can't be used with --format")
		}
		if !isTerminal(env.Stdout) || !isTerminal(os.Stdin) {
			return usagef("--interactive needs a terminal")
		}
		cfg := tui.Config{Dir: dir, Period: p, Now: env.Now}
		if sel, ok := opts.selection(); ok {
			cfg.Detail = &sel
		}
		return tui.Run(cmd.Context(), cfg)
	}

	loaded, err := record.Load(dir, p.First, p.Last)
	if err != nil {
		return err
	}
	if loaded.Skipped > 0 {
		fmt.Fprintf(env.Stderr, "timesheet: skipped %d unreadable record(s) in %s\n", loaded.Skipped, dir)
	}
	days := sheet.Build(loaded.Records)
	sel, selected := opts.selection()

	switch strings.ToLower(opts.format) {
	case "json", "csv":
		if selected {
			days = sheet.Filter(days, sel)
		}
		if strings.EqualFold(opts.format, "csv") {
			return export.CSV(env.Stdout, days)
		}
		return export.JSON(env.Stdout, p, days)
	case "text":
		theme := render.NewTheme(profile > colorprofile.ASCII, sheet.Sessions(days))
		out := &colorprofile.Writer{Forward: env.Stdout, Profile: profile}
		w := width(env.Stdout, env)
		report := render.Report(theme, p, days, render.Options{Width: w, Timetables: opts.timetable, Dir: dir})
		if selected {
			report = theme.Heading(p.Title()) + "\n\n" + render.DetailText(render.Detail(theme, days, sel, w))
		}
		_, err := io.WriteString(out, report)
		return err
	}
	return usagef("--format takes text, json or csv, not %q", opts.format)
}

// selection is the work --session and --work narrow the report to, if either is set.
func (o options) selection() (sheet.Selection, bool) {
	if o.session == "" && o.work == "" {
		return sheet.Selection{}, false
	}
	return sheet.SelectNamed(o.session, o.work), true
}

// IsUsage says whether an error came from how the command was called.
func IsUsage(err error) bool {
	var usage UsageError
	return errors.As(err, &usage)
}
