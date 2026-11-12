// Package record reads the monthly CSV files tmux-telemetry writes.
package record

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Event is what a record marks: focus moving to a session, or tmux detached.
type Event int

const (
	Focus Event = iota
	Away
)

// Record is one row: the focused session and its branch or folder at a moment.
type Record struct {
	Time    time.Time
	Event   Event
	Session string
	Context string
}

// Date is a calendar day, held as midnight UTC so days compare without time zones.
type Date = time.Time

// DateOf is the day a moment falls on, by the clock it was recorded with.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Dir is where tmux-telemetry keeps its records.
func Dir() string {
	if dir := os.Getenv("TELEMETRY_DIR"); dir != "" {
		return dir
	}
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "telemetry")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "telemetry")
	}
	return filepath.Join(home, ".local", "state", "telemetry")
}

// Result holds the records read, and how many rows were skipped as unreadable.
type Result struct {
	Records []Record
	Skipped int
}

// Parse reads one month's CSV. Rows that don't parse, such as one cut short by a
// crash, are counted and skipped rather than failing the whole month.
func Parse(r io.Reader) (Result, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true

	var result Result
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		var parseErr *csv.ParseError
		if errors.As(err, &parseErr) {
			result.Skipped++
			continue
		}
		if err != nil {
			return result, err
		}

		// concurrent writers can each add a header to a new month
		if row[0] == "timestamp" {
			continue
		}
		rec, ok := parseRow(row)
		if !ok {
			result.Skipped++
			continue
		}
		result.Records = append(result.Records, rec)
	}
}

func parseRow(row []string) (Record, bool) {
	if len(row) != 4 {
		return Record{}, false
	}
	t, err := time.Parse(time.RFC3339, row[0])
	if err != nil {
		return Record{}, false
	}

	rec := Record{Time: t, Session: row[2], Context: row[3]}
	switch row[1] {
	case "focus":
		if rec.Session == "" {
			return Record{}, false
		}
	case "away":
		rec.Event = Away
		rec.Session, rec.Context = "", ""
	default:
		return Record{}, false
	}
	return rec, true
}

// Load reads every record on the days from first to last, inclusive, in time order.
func Load(dir string, first, last Date) (Result, error) {
	var result Result

	// a day's records sit in its own month's file, whatever the offset
	for month := time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, time.UTC); !month.After(last); month = month.AddDate(0, 1, 0) {
		path := filepath.Join(dir, month.Format("2006-01")+".csv")
		file, err := os.Open(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return result, err
		}
		got, err := Parse(file)
		file.Close()
		if err != nil {
			return result, fmt.Errorf("%s: %w", path, err)
		}

		result.Skipped += got.Skipped
		for _, rec := range got.Records {
			if day := DateOf(rec.Time); !day.Before(first) && !day.After(last) {
				result.Records = append(result.Records, rec)
			}
		}
	}

	sort.SliceStable(result.Records, func(i, j int) bool {
		return result.Records[i].Time.Before(result.Records[j].Time)
	})
	return result, nil
}
