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

func main() {
	err := fang.Execute(context.Background(), cli.New(cli.System()),
		fang.WithVersion(buildVersion()),
		fang.WithNotifySignal(os.Interrupt),
		fang.WithErrorHandler(printError),
	)
	switch {
	case err == nil:
	case cli.IsUsage(err):
		os.Exit(2)
	default:
		os.Exit(1)
	}
}
