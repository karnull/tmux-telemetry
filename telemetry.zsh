# Source from .zshrc. Every prompt logs focus, so cd through any alias, and git switch, are caught

[[ -n $TELEMETRY_HOME ]] || return 0

alias telemetry="$TELEMETRY_HOME/scripts/telemetry.sh"

_telemetry_record() { "$TELEMETRY_HOME/scripts/telemetry.sh" record "$TMUX_PANE" &>/dev/null &! }

if [[ -n $TMUX ]]; then
    autoload -Uz add-zsh-hook
    add-zsh-hook precmd _telemetry_record
fi
