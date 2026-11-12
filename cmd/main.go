/*
Copyright (c) 2026 Apache-2.0 license

@file    main.go
@author  karnull
@brief   Main entry point
*/

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "timesheet",
	Short: "Breaks tmux-telemetry records down into a timesheet",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO add code...
		fmt.Println("Timesheet")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
