/*
Copyright (c) 2026 Apache-2.0 license

@file    main.go
@author  karnull
@brief   Main entry point
*/

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"

	"github.com/karnull/tmux-telemetry/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=..."
var version = ""

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

// errors go out the POSIX way, as "timesheet: message", which scripts can grep
func printError(w io.Writer, _ fang.Styles, err error) {
	fmt.Fprintf(w, "timesheet: %v\n", err)
	if cli.IsUsage(err) {
		fmt.Fprintln(w, "Try 'timesheet --help' for more information.")
	}
}

// colours plain on the terminal's own background, rather than set in blocks
func plainScheme(c lipgloss.LightDarkFunc) fang.ColorScheme {
	scheme := fang.DefaultColorScheme(c)
	scheme.Codeblock = lipgloss.NoColor{}
	return scheme
}

func main() {
	// help pads its lines out to its width, which shows once copied or piped
	out := cli.Trim(os.Stdout)
	root := cli.New(cli.System())
	root.SetOut(out)

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(buildVersion()),
		fang.WithNotifySignal(os.Interrupt),
		fang.WithErrorHandler(printError),
		fang.WithColorSchemeFunc(plainScheme),
	)
	_ = out.Flush()
	switch {
	case err == nil:
	case cli.IsUsage(err):
		os.Exit(2)
	default:
		os.Exit(1)
	}
}
