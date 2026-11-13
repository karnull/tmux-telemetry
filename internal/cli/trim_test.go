package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTrimTakesOffTrailingSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	out := Trim(f)
	for _, chunk := range []string{"  USAGE  \n", "    \x1b[1mtimesheet\x1b[m   \x1b[0m  \n", "     \n", "split ", "line   ", "\n", "tail   "} {
		// io.WriteString, as the colour writer in front of help uses
		if _, err := io.WriteString(out, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := out.Flush(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, _ := os.ReadFile(path)
	want := "  USAGE\n    \x1b[1mtimesheet\x1b[m\x1b[0m\n\nsplit line\ntail"
	if string(got) != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
