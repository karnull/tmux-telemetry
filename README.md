# tmux-telemetry

Keeps a log of where the working day went, without changing how you work. It
assumes one tmux session per project: every switch between sessions, windows
and panes is logged with the session name and the git branch in focus, or the
folder when outside a repository. A shell hook also logs each prompt, so `cd`
through any alias, and `git switch`, are picked up without wrapping either.

Panes in the background, sessions with no client attached and ignored sessions
log nothing. Detaching tmux counts as away.

Rename a window to say what the time is going on, and that goes down in place of
the branch, whatever the pane shows: in a meeting, rename the window `meeting`
and carry on doodling, and the record says Meeting. Bench, Research, Meeting
and Hardware are the names, in any case, and `!meeting` goes down as External.
Windows named `lunch` and `afk` record a lunch break and time away, which belong
to no session. timesheet lists them but leaves them out of every total.

### When the day ends

A watcher runs beside tmux. The day ends when tmux stops, or where the records
stopped when the machine was turned off or hibernated. A gap shorter than
`@telemetry-restart` minutes, from restarting the machine or tmux, goes down as
AFK instead.

### Quiet spells

After `@telemetry-idle` minutes with no key pressed in tmux, a popup asks whether
anyone is still there, unless the window in focus is named `afk` or `lunch`. No, or no answer in two minutes, records the time since
the last key as UNATTENDED. On coming back, a second popup asks where you were:
`c` to carry on with whatever was in focus before, as one stretch with it, `a`
for AFK, `l` for lunch, `m` for Meeting, `e` for External, or anything typed,
which goes to the session in focus. Enter alone leaves it UNATTENDED, to fill in
later from `timesheet -i`, which takes the same answers.

`timesheet`, built from this repository, turns the records into a report: the
time spent on each session and branch, a pie chart of the sessions, a timeline
of every day, and an interactive view to browse it all.

[![Copier](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/copier-org/copier/master/img/badge/badge-grayscale-inverted-border-orange.json)](https://github.com/karshosphere/template-go)


## Install

With [tpm](https://github.com/tmux-plugins/tpm), in `tmux.conf`:

```tmux
set -g @plugin 'karnull/tmux-telemetry'
```

Or from a clone:

```tmux
run-shell /path/to/tmux-telemetry/telemetry.tmux
```

Then, for the prompt hook, in `.zshrc`:

```zsh
[[ -n $TELEMETRY_HOME ]] && source "$TELEMETRY_HOME/telemetry.zsh"
```

The plugin sets `TELEMETRY_HOME` in the tmux environment, so it only reaches
shells started after the plugin loads. Only zsh is supported.

For `timesheet`, take the binary for your system from the latest release, or
build it with `make build`, which needs Go 1.26 or later.


## Options

| Option               | Default                           | What it does                                                        |
|----------------------|-----------------------------------|---------------------------------------------------------------------|
| `@telemetry-ignore`  | empty                             | sessions never logged, space separated                              |
| `@telemetry-trunk`   | `main master`                     | branches recorded by folder name, space separated                   |
| `@telemetry-labels`  | `Bench Research Meeting Hardware` | window names recorded in place of the branch, space separated       |
| `@telemetry-idle`    | `5`                               | minutes without a key pressed before asking whether anyone is there |
| `@telemetry-restart` | `15`                              | minutes a gap can last and still be a restart, recorded as AFK      |

The records go to `$XDG_STATE_HOME/telemetry`, or to `$TELEMETRY_DIR` when set.


## Records

Each month is one CSV file, such as `2026-11.csv`:

```csv
timestamp,event,session,context
2026-11-10T09:05:12+11:00,focus,Minecraft,chore/get-started
2026-11-10T12:01:40+11:00,away,,
2026-11-10T13:32:08+11:00,focus,Cyberpunk,~/notes
```

| Column      | What it holds                                                                                                                                    |
|-------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| `timestamp` | RFC 3339, with the offset                                                                                                                        |
| `event`     | `focus` when a pane gains focus or shows a prompt, `away` on detach, `lunch` and `afk` for those windows and a restart, `idle` for a quiet spell |
| `session`   | the tmux session                                                                                                                                 |
| `context`   | the window's label; else the git branch; the folder name on the trunk; the folder outside git; empty at home                                     |

## timesheet

Without a period, `timesheet` reports the previous week.

```sh
timesheet --day --current        # today so far
timesheet --day                  # yesterday
timesheet --day 11/04/2026       # the 11th of April
timesheet --week 25,2026         # ISO week 25 of 2026
timesheet --week --current       # this week so far
timesheet --month "April 2026"
timesheet 2026-W25               # a period on its own is read by its form
timesheet --since monday         # a range, to today unless --until says
```

`--day`, `--week` and `--month` take a period of their own, or pair with
`--current` or `--previous` (the default) on their own. Short options group the
POSIX way, so `-cd` is today and `-pm` the previous month.

| Period    | Forms it takes                                                      |
|-----------|---------------------------------------------------------------------|
| `--day`   | `11/04/2026`, `11/04`, `2026-04-11`, `today`, `yesterday`, `monday` |
| `--week`  | `25,2026`, `2026-W25`, `W25`, `25`, or any day in the week          |
| `--month` | `04/2026`, `2026-04`, `April 2026`, `april`, `4`, or any day in it  |

Days go day first, so `11/04/2026` is the 11th of April. Weeks are ISO weeks,
Monday to Sunday, and a weekday name is the latest one, today included. Without
`--day`, `--week` or `--month`, a bare number could be any of them and is
refused, and `04/2026` reads as a month.

| Option                | What it does                                                     |
|-----------------------|------------------------------------------------------------------|
| `-s`, `--since`       | first day of a range, in any form `--day` takes                  |
| `-u`, `--until`       | last day of a range, today unless set                            |
| `--session`           | only this session, or `afk` or `lunch`, with every stretch of it |
| `--work`              | only this branch, folder or label, with every stretch of it      |
| `-t`, `--timetable`   | add each day's timetable to the report                           |
| `-i`, `--interactive` | browse the report in the terminal                                |
| `-f`, `--format`      | `text`, `json` for the whole report, `csv` for one row an entry  |
| `--colour`            | `auto`, `always` or `never`; `NO_COLOR` is honoured              |
| `--dir`               | where the records are                                            |

Without colour, from `NO_COLOR`, a pipe or `--colour never`, sessions stand
apart by glyph instead.

Errors go to stderr as `timesheet: message`. A mistake in the options exits with
status 2, and any other failure with 1. `timesheet completion zsh` (or `bash`,
`fish`, `powershell`) prints shell completions, and `timesheet man` a manual page.

### How the time is counted

* A day runs from its first record to its last, with no fixed hours.
* Records in a row with the same session and branch make one stretch, which lasts
  until the next change. The last stretch of a day ends at its last record.
* Time at home, or anywhere for less than a minute, is only passing through: it
  goes to the next place the work stays, from when it started. A quick hop away
  and straight back leaves one stretch, and passing through just before a detach
  or the end of the day isn't counted.
* Detaching tmux counts as away until the next record.
* The totals table keeps each session's rows together, the sessions longest first
  and their work longest first within each.
* AFK, lunch breaks and UNATTENDED time show in the totals table, the timetables
  and the timeline, AFK as a faint `z`, lunch as faint dots and UNATTENDED as
  faint question marks, but are left out of each session's total, the pie chart
  and every day's total. Each has its own total in the report's heading, and JSON
  and CSV mark them with `counted` false.

### Interactive

`timesheet -i` opens on the period the options pick, and reads the records again
every 30 seconds while the period is still going. With `--session` or `--work`
it opens straight onto that work's detail.

Click anything to see every stretch spent on it across the period: a totals row,
a legend entry for the whole session, a day's entry, or a spot on any timeline.
The detail lists each day's stretches, such as `12:35 - 17:45`, beside a timeline
marking them among the rest of the day. A session's detail lists its work, and
clicking one opens that work's own. The detail stays open moving between
periods, so the same work can be followed week to week.

| Key                | What it does                                  |
|--------------------|-----------------------------------------------|
| `tab`, `1` `2` `3` | summary, days, timeline                       |
| `←` `→`, `h` `l`   | the period before or after                    |
| `d` `w` `m`        | switch to a day, a week or a month            |
| `c`                | back to the current period                    |
| `↑` `↓`, `j` `k`   | move through the totals or the days           |
| `J` `K`            | move through the day's entries                |
| `enter`, click     | every stretch of the row under the cursor     |
| `s`                | every stretch of its whole session            |
| `e`                | say where an UNATTENDED entry went            |
| `esc`              | back a step, or close the detail              |
| `r`                | read the records again                        |
| `?`                | every key                                     |
| `q`                | quit                                          |


## Development

```sh
make build            # timesheet into build/
make test             # timesheet's tests, then the plugin's in tests/
make format           # format in place; make format-dry-run lists what needs it
make release          # cross-compile each release target into dist/
```

`tests/record.sh`, `tests/watch.sh` and `tests/idle.sh` drive a private tmux
server, and need tmux, zsh, git and python3, all of which the dev container has. CI runs the four make targets in
the dev container on every pull request, and publishes a release on each merge
to main, with its version worked out from the commit messages.

* [Project Structure](docs/project_structure.md): repository layout and package
  responsibilities

The Go side comes from [template-go](https://github.com/karshosphere/template-go).
To bring in a newer release of it:

```sh
copier update --defaults --answers-file .copier-go-answers.yml
```


## Licence

Apache 2.0, see [LICENSE](LICENSE).
