# ghosttype zsh integration script
function ghosttype_predict() {
  local input="$BUFFER"
  local suggestion=$(ghosttype "$input" | fzf --prompt="ghosttype> " | head -n1)
  
  if [[ -n $suggestion ]]; then
    BUFFER="$suggestion"
    CURSOR=${#BUFFER}
    zle reset-prompt  
  fi
}   
  
zle -N ghosttype_predict
bindkey '^P' ghosttype_predict
