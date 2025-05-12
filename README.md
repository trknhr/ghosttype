# ghosttype

## ⌨️ Fuzzy Completion with fzf

**ghosttype** is a smart command suggestion tool for your terminal.  
It learns from your shell history and suggests the next most likely command using a lightweight Markov chain model.  
With fuzzy selection powered by `fzf`, ghosttype makes completing commands fast, intuitive, and shell-agnostic.

## 🚧 Status: Under Development

This project is still in an early development phase.  
**Functionality may be unstable or incomplete. Use at your own risk.**

Your feedback and contributions are welcome!

## 🚀 Quick Demo

```zsh
$ git ch▍    # Press Ctrl+P
> git checkout main
  git cherry-pick HEAD
  git checkout -b feature
```

## ✨ Features

- 📚 Learns from your `~/.zsh_history` or `~/.bash_history`
- 🧠 Predicts likely next tokens using Markov transitions
- 🔎 Fuzzy picker with `fzf` to choose completions interactively
- ⚡ Instant zsh integration with simple keybinding
- 🧩 Shell-agnostic CLI (can be integrated with bash, fish, etc.)

## 🛠 Installation

### 1. Install ghosttype

```bash
go install github.com/trknhr/ghosttype@latest
```

### 2. Install fzf (if not already installed)

```bash
brew install fzf
```

---

## 🧬 Zsh Integration

Add this to your `~/.zshrc`:

```zsh
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
```

Then apply it:

```bash
source ~/.zshrc
```

---

## 🧠 How It Works

1. Parses your shell history (e.g., `.zsh_history`)
2. Builds a Markov chain of command token transitions
3. Given a partial input (like `git `), it:
   - Finds the last token (`git`)
   - Predicts likely next tokens (`checkout`, `cherry-pick`, etc.)
   - Prepends the input and prints full completions
4. `fzf` lets you pick one

---

## 🗂 Directory Structure

```
.
├── cmd/            # CLI command logic (cobra)
├── history/        # Shell history loaders
├── marcov/         # Markov model
├── script/         # Shell integration scripts
├── main.go
├── go.mod
└── README.md
```

---

## 📜 License

Apache-2.0  
See [LICENSE](./LICENSE) for full terms.
