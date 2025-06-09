# ghosttype

## ⌨️ Terminal Command Prediction

**Ghosttype** is your AI-powered command assistant for the terminal.  
It learns how you work — from your command history, project context, and shell configuration — and predicts what you're most likely to type next.

Using a hybrid of traditional and AI-enhanced models, Ghosttype intelligently suggests your next move with:

- 🔁 **Markov chains** – learning the flow of your typical command sequences  
- 📊 **Frequency analysis** – surfacing your most common commands quickly  
- 🧠 **LLM-based embeddings** – understanding semantic similarity via vector search  
- 💾 **Shell aliases** – integrating your custom shortcuts  
- 📦 **Project context awareness** – reading from `Makefile`, `package.json`, `pom.xml`, and more

> It’s like having autocomplete — but for the way *you* use the terminal.

## 🚧 Status: Active Development

Ghosttype is still under active development.
Expect occasional breaking changes. Contributions and issue reports are welcome!

## 📊 Performance & Benchmarks

**Current Performance vs. Popular Tools**

We regularly benchmark Ghosttype against established command-line tools to track our progress:
```
🏆 BENCHMARK RESULTS (600 test cases)
┌─────────────┬─────────┬─────────┬─────────┬───────────┬──────────┐
│ Tool        │ Top-1   │ Top-10  │ Avg Time│ P95 Time  │ Errors   │
├─────────────┼─────────┼─────────┼─────────┼───────────┼──────────┤
│ 👑 ghosttype │   20.0% │   36.0% │ 832ms   │    2.1s   │    0.0%  │
│ fzf         │    8.0% │   14.0% │  19ms   │   24ms    │   54.0%  │
└─────────────┴─────────┴─────────┴─────────┴───────────┴──────────┘
```

**What we're doing well:**
- **2.5x more accurate** command predictions than traditional fuzzy finders
- **Zero errors** vs 54% error rate in string-based matching
- **Better semantic understanding** of command intent

**What we're working on:**
- **Latency optimization**: Current ~800ms response time needs improvement for real-time use
- **Model efficiency**: Exploring lighter models and caching strategies  
- **Progressive loading**: Show fast results immediately, then enhance with AI suggestions
- **Hybrid approach**: Instant prefix matching for short inputs, AI for complex queries

## 🚀 Demo

```zsh
$ git ch▍    # Press Ctrl+P (zsh Integration)

> git checkout main                                       
  git checkout add-slim-version                           
  git checkout hoge                                       
```

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

This will install the ghosttype command to your $GOBIN (usually ~/go/bin).

## 🖥️ Zsh Integration

Add the following to your .zshrc:

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

Then reload your shell:

```bash
source ~/.zshrc
```

Now press `Ctrl+P` in your terminal to trigger Ghosttype suggestions.

## 🧠 Enable LLM-Powered Suggestions (via Ollama)

Ghosttype supports **LLM-based predictions and vector embeddings** powered by [Ollama](https://ollama.com/).

To use these features, follow the steps below:

### 1. Install Ollama

Download and install from the official site:  
👉 [https://ollama.com/download](https://ollama.com/download)

Verify installation:

```bash
ollama --version
``` 

### 2. Pull required models
Ghosttype uses the following models:

`llama3.2` — for next-command prediction

`nomic-embed-text` — for semantic similarity via embedding

Download the models:

```bash
ollama run llama3.2           # Starts and downloads the LLM model
ollama pull nomic-embed-text  # Downloads the embedding model
```

ℹ️ ollama run llama3.2 must be running in the background to enable LLM-powered suggestions.

You can run it in a separate terminal window:
Once Ollama is running and the models are downloaded, Ghosttype will automatically use them to enhance prediction accuracy.

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
