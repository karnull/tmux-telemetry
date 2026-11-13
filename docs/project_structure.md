# tmux-telemetry Structure

## Overview

Two halves share the repository. The tmux plugin records which session and
branch has focus, as monthly CSV files. timesheet, in Go, reads those records,
works out what was worked on and when, and shows it as text, JSON, CSV or an
interactive TUI. Each step of timesheet is its own package, so the rules for
counting time are tested apart from how they are shown.


## Top-Level Layout

| Path                    | What it holds                                                  |
|-------------------------|----------------------------------------------------------------|
| `telemetry.tmux`        | the plugin's entry point, which tpm runs: it sets the hooks    |
| `telemetry.zsh`         | the prompt hook, sourced from `.zshrc`                         |
| `scripts/telemetry.sh`  | writes the records                                             |
| `scripts/release.sh`    | timesheet's release build, from the template                   |
| `tests/`                | the plugin's tests, which `make test` runs after timesheet's   |
| `cmd/`                  | timesheet's entry point: version, error output and exit status |
| `internal/`             | the rest of timesheet, one package per step                    |
| `docs/`                 | this file                                                      |


## Runtime Package

| Package             | What it does                                                           |
|---------------------|------------------------------------------------------------------------|
| `internal/record`   | reads the records, skipping rows a crash cut short                     |
| `internal/sheet`    | turns records into each day's entries, leaving AFK and lunch uncounted |
| `internal/period`   | days, ISO weeks, months and ranges: reading them, and stepping through |
| `internal/render`   | Lip Gloss tables, the pie chart, timelines and the text report         |
| `internal/export`   | JSON and CSV output                                                    |
| `internal/cli`      | the command and its options, built on Cobra and run through Fang       |
| `internal/tui`      | the interactive view on Bubble Tea, with Lip Gloss layers to click on  |

Records flow one way: `record` → `sheet` → `render`, `export` or `tui`, with
`period` deciding which days to read.
