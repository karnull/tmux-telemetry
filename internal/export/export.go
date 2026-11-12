// Package export writes reports for other programs: JSON for the whole report, and
// CSV with one row per entry.
package export

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/karnull/tmux-telemetry/internal/period"
	"github.com/karnull/tmux-telemetry/internal/sheet"
)

type jsonPeriod struct {
	Unit  string `json:"unit"`
	First string `json:"first"`
	Last  string `json:"last"`
	Title string `json:"title"`
}

type jsonEntry struct {
	Start   string `json:"start"`
	End     string `json:"end"`
	Session string `json:"session"`
	Context string `json:"context"`
	Minutes int    `json:"minutes"`
}

type jsonDay struct {
	Date    string      `json:"date"`
	Minutes int         `json:"minutes"`
	Entries []jsonEntry `json:"entries"`
}

type jsonTotal struct {
	Session string  `json:"session"`
	Context string  `json:"context,omitempty"`
	Minutes int     `json:"minutes"`
	Share   float64 `json:"share"`
}

type jsonReport struct {
	Period   jsonPeriod  `json:"period"`
	Minutes  int         `json:"minutes"`
	Days     []jsonDay   `json:"days"`
	Totals   []jsonTotal `json:"totals"`
	Sessions []jsonTotal `json:"sessions"`
}

func minutes(d time.Duration) int { return int(d.Round(time.Minute) / time.Minute) }

func totals(list []sheet.Total, grand time.Duration) []jsonTotal {
	out := make([]jsonTotal, 0, len(list))
	for _, t := range list {
		share := 0.0
		if grand > 0 {
			share = t.Duration.Seconds() / grand.Seconds()
		}
		out = append(out, jsonTotal{Session: t.Session, Context: t.Context, Minutes: minutes(t.Duration), Share: share})
	}
	return out
}

// JSON writes the whole report, durations in minutes and dates as YYYY-MM-DD.
func JSON(w io.Writer, p period.Period, days []sheet.Day) error {
	grand := sheet.Grand(days)
	report := jsonReport{
		Period: jsonPeriod{
			Unit:  p.Unit.String(),
			First: p.First.Format(time.DateOnly),
			Last:  p.Last.Format(time.DateOnly),
			Title: p.Title(),
		},
		Minutes:  minutes(grand),
		Days:     []jsonDay{},
		Totals:   totals(sheet.Totals(days), grand),
		Sessions: totals(sheet.Sessions(days), grand),
	}
	for _, d := range days {
		day := jsonDay{Date: d.Date.Format(time.DateOnly), Minutes: minutes(d.Total()), Entries: []jsonEntry{}}
		for _, e := range d.Entries {
			day.Entries = append(day.Entries, jsonEntry{
				Start:   sheet.Clock(e.Start),
				End:     sheet.Clock(e.End),
				Session: e.Session,
				Context: e.Context,
				Minutes: minutes(e.Duration()),
			})
		}
		report.Days = append(report.Days, day)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// CSV writes one row per entry, ready for a spreadsheet.
func CSV(w io.Writer, days []sheet.Day) error {
	out := csv.NewWriter(w)
	if err := out.Write([]string{"date", "start", "end", "session", "context", "minutes"}); err != nil {
		return err
	}
	for _, d := range days {
		for _, e := range d.Entries {
			row := []string{
				d.Date.Format(time.DateOnly),
				sheet.Clock(e.Start),
				sheet.Clock(e.End),
				e.Session,
				e.Context,
				strconv.Itoa(minutes(e.Duration())),
			}
			if err := out.Write(row); err != nil {
				return err
			}
		}
	}
	out.Flush()
	return out.Error()
}
