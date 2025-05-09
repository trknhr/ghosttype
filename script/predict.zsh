# ghosttype zsh integration script for suggestions
function predict() {
  local input="$BUFFER"
  local suggestion=$(ghosttype "$input" | fzf --prompt="ghosttype suggestions: ")

  if [[ -n $suggestion ]]; then
    BUFFER="$suggestion"
    CURSOR=${#BUFFER}
    zle reset-prompt
  fi
}

zle -N predict
bindkey '^P' predict  # Trigger suggestion with Ctrl+P
