# ghosttype

## ⌨️ Terminal Command Prediction

**ghosttype** is a smart command suggestion tool for your terminal.
It learns from your shell history and context, and suggests the next most likely command using a combination of:

* 🔁 Markov chains
* 📊 Frequency analysis
* 🧠 Embedding similarity
* 💾 Aliases from your shell config
* 📦 Project context (e.g. npm, Makefile, pom.xml)

## 🚧 Status: Active Development

Ghosttype is still under active development.
Expect occasional breaking changes. Contributions and issue reports are welcome!

---

## 🚀 Demo

```zsh
$ git ch▍    # Press Ctrl+P (zsh Integration)

   Suggestions                                            
                                                          
  32 items                                                
                                                          
> git checkout main                                       
  git checkout add-slim-version                           
  git checkout hoge                                       
```

---

## ✨ Features

* 📚 Learns from `~/.zsh_history` or `~/.bash_history`
* 🤖 Embeds historical commands via LLM-powered vector search
* 🧠 Predicts likely next commands using multiple models (Markov, freq, embedding, etc.)
* 📂 Context-aware suggestions from `Makefile`, `package.json`, `pom.xml`, etc.
* ⚡ Zsh keybinding integration

---

## 🛠 Installation

### 1. Install ghosttype

```bash
go install github.com/trknhr/ghosttype@latest
```

## 🖥️ Zsh Integration

```zsh
# Predict a command using ghosttype + TUI, then replace current shell input with the selection
function ghosttype_predict() {
  local result=$(ghosttype "$BUFFER")
  if [[ -n "$result" ]]; then
    BUFFER="$result"
    CURSOR=${#BUFFER}
    zle reset-prompt
  fi
}
zle -N ghosttype_predict
bindkey '^p' ghosttype_predict
```

## 🧠 Architecture

Ghosttype uses an ensemble of models:

* `markov`: Lightweight transition-based predictor
* `freq`: Frequency-based suggestion engine
* `alias`: Shell aliases from `.zshrc`/`.bashrc`
* `context`: Targets from `Makefile`, `package.json`, `pom.xml`, etc.
* `embedding`: LLM-generated vector search powered by `ollama`

All models implement a unified `SuggestModel` interface and are combined via `ensemble.Model`.

## 🗂 Project Structure

```
.
├── cmd/            # CLI (tui, suggest, root)
├── history/        # Loaders for bash/zsh history
├── model/          # All prediction models
├── internal/       # Logging, utils, alias sync
├── ollama/         # LLM/embedding interface
├── parser/         # RC and alias parsing
├── script/         # Shell helper scripts
├── main.go
└── go.mod
```


## 📜 License

Apache-2.0
See [LICENSE](./LICENSE) for full terms.
