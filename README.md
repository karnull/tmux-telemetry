# tmux-telemetry

Keeps a log of where the working day went, without changing how you work. It
assumes one tmux session per project: every switch between sessions, windows
and panes is logged with the session name and the git branch in focus, or the
folder when outside a repository. A shell hook also logs each prompt, so `cd`
through any alias, and `git switch`, are picked up without wrapping either.

Panes in the background, sessions with no client attached and ignored sessions
log nothing. Detaching tmux counts as away.


## Report

```
Tuesday 10/11/2026

09:00 - 12:00 | Roblox        | release-1.0           |
13:32 - 15:01 | Minecraft     | chore/get-started     |
15:01 - 15:13 | Minecraft     | chore/auto-update     |

Total:
Roblox        | release-1.0           | 3:00  |
Minecraft     | chore/get-started     | 1:29  |
Minecraft     | chore/auto-update     | 0:12  |
```

| Command          | What it shows             |
|------------------|---------------------------|
| `telemetry`      | this week, from Monday    |
| `telemetry all`  | every recorded day        |

Each report also rewrites the full log to `telemetry.txt`, beside the daily
records in `~/.local/state/telemetry/`.

How the time is counted:

* records in a row with the same session and branch make one stretch, which
  lasts until the next change, and the last stretch of a day ends at its last
  record
* the first stretch of the morning counts from the start of the day, even if
  the terminal came out later, and anything earlier is dropped
* a stretch still running at lunch stops there, and work picks up again at the
  first record after it


## Install

With [tpm](https://github.com/tmux-plugins/tpm), in `tmux.conf`:

```tmux
set -g @plugin 'karnull/tmux-telemetry'
```

Or from a clone:

```tmux
run-shell /path/to/tmux-telemetry/telemetry.tmux
```

Then, for the prompt hook and the `telemetry` command, in `.zshrc`:

```zsh
[[ -n $TELEMETRY_HOME ]] && source "$TELEMETRY_HOME/telemetry.zsh"
```

The plugin sets `TELEMETRY_HOME` in the tmux environment, so it only reaches
shells started after the plugin loads. Only zsh is supported.


## Options

| Option                 | Default       | What it does                            |
|------------------------|---------------|-----------------------------------------|
| `@telemetry-day-start` | `09:00`       | when the working day starts             |
| `@telemetry-lunch`     | `12:00-13:00` | time left out of the day                |
| `@telemetry-ignore`    | empty         | sessions never logged, space separated  |

The records go to `$XDG_STATE_HOME/telemetry`, or to `$TELEMETRY_DIR` when set.


## Tests

```sh
tests/report.sh   # the report matches a known day
tests/record.sh   # a private tmux server logs what it should
```

`tests/record.sh` needs `python3` to attach a client on a pseudo-terminal.


## Licence

Apache 2.0, see [LICENSE](LICENSE).
