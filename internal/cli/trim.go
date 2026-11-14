package cli

import (
	"bytes"
	"os"
	"regexp"
)

// TrimmedFile writes to a file with the spaces at the end of each line taken off,
// which help pads out to its full width. It passes on the file's descriptor, so
// colour is still detected from the terminal behind it. The file is a field, not
// embedded, so its own WriteString can't be reached around the trimming.
type TrimmedFile struct {
	file    *os.File
	pending []byte
}

// Trim wraps a file so each line it is sent loses its trailing spaces.
func Trim(f *os.File) *TrimmedFile { return &TrimmedFile{file: f} }

// Fd is the file's descriptor, so the terminal behind it can be asked about.
func (t *TrimmedFile) Fd() uintptr { return t.file.Fd() }

// Read reads from the file.
func (t *TrimmedFile) Read(p []byte) (int, error) { return t.file.Read(p) }

// Close writes out what is pending, and closes the file.
func (t *TrimmedFile) Close() error {
	if err := t.Flush(); err != nil {
		return err
	}
	return t.file.Close()
}

// the spaces may sit before the codes that reset a line's style, which stay
var trailing = regexp.MustCompile(`[ \t]+((?:\x1b\[[0-9;:]*m)*)$`)

func trimLine(line []byte) []byte {
	for {
		trimmed := trailing.ReplaceAll(line, []byte("$1"))
		if bytes.Equal(trimmed, line) {
			return trimmed
		}
		line = trimmed
	}
}

// Write passes on every whole line, trimmed, and keeps any part line for later.
func (t *TrimmedFile) Write(p []byte) (int, error) {
	t.pending = append(t.pending, p...)
	for {
		i := bytes.IndexByte(t.pending, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := append(trimLine(bytes.Clone(t.pending[:i])), '\n')
		t.pending = t.pending[i+1:]
		if _, err := t.file.Write(line); err != nil {
			return 0, err
		}
	}
}

// Flush writes out a last line that never got its newline.
func (t *TrimmedFile) Flush() error {
	if len(t.pending) == 0 {
		return nil
	}
	_, err := t.file.Write(trimLine(t.pending))
	t.pending = nil
	return err
}
