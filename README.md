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

> 0.3.0 switches the CLI over to the Rust implementation and ships prebuilt binaries via GitHub Releases.

## 📊 Performance & Benchmarks

We regularly benchmark Ghosttype against established command-line tools to track our progress:
```
┌─────────────┬─────────┬─────────┬─────────┬───────────┬──────────┐
│ Tool        │ Top-1   │ Top-10  │ Avg Time│ P95 Time  │ Errors   │
├─────────────┼─────────┼─────────┼─────────┼───────────┼──────────┤
│ 👑 ghosttype │   16.0% │   31.0% │ 158.483ms │ 255.674ms │     0.5% │
│ fzf         │    7.5% │   13.5% │ 10.846ms │  15.67ms │    41.5% │
└─────────────┴─────────┴─────────┴─────────┴───────────┴──────────┘

🥇 WINNERS BY METRIC:
  Best Top-1 Accuracy: ghosttype
  Best Top-10 Accuracy: ghosttype
  Fastest Average Response: fzf
  Best P95 Latency: fzf
  Most Reliable: ghosttype

💡 GHOSTTYPE ADVANTAGES:
  ✅ 2x more accurate than fzf (16.0% vs 7.5%)
```

**What we're doing well:**
- **2x more accurate** command predictions than traditional fuzzy finders
- **Zero errors** vs 54% error rate in string-based matching
- **Better semantic understanding** of command intent

**What we're working on:**
- **Latency optimization**: Current ~800ms response time needs improvement for real-time use
- **Model efficiency**: Exploring lighter models and caching strategies  
- **Progressive loading**: Show fast results immediately, then enhance with AI suggestions
- **Hybrid approach**: Instant prefix matching for short inputs, AI for complex queries
- **Deeper contextual understanding**: Providing more relevant suggestions by analyzing the current directory's files, git status, and recently executed commands.
- **Intelligent error correction**: Suggesting corrections for typos or common errors (e.g., correcting gti status to git status).

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

### Option 1: Install Script (Recommended)

```bash
curl -sL https://raw.githubusercontent.com/trknhr/ghosttype/main/script/install.sh | bash
```

The script downloads the latest release archive for your OS/arch and installs a `ghosttype` binary to `/usr/local/bin`.

### Option 2: Build From Source (Rust)

```bash
cd rust
cargo install --path . --locked
```

This builds the Rust CLI and installs it to your cargo bin directory (usually `~/.cargo/bin`). You can also run `cargo build --release` and pick up the binary from `rust/target/release/ghosttype`.

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

## 🧠 Embeddings + LLM setup (llama.cpp)

Ghosttype uses the `llama-embedding` binary from [`llama.cpp`](https://github.com/ggerganov/llama.cpp) for vector embeddings.

1) Install llama.cpp

macOS (Homebrew):

```bash
brew install llama.cpp
```

2) Get an embedding model (GGUF)

Download a compatible GGUF embedding model (e.g., `nomic-embed-text`) and note its path. You can place it anywhere; pass the path below.

3) Run ghosttype with embeddings enabled (default)

```bash
ghosttype tui --embedding-model /path/to/your/model.gguf
```

Common flags:

- `--enable-embedding=false` to skip embeddings entirely
- `--enable-llm` / `--llm-model <path>` for the (optional) LLM generator

Environment overrides:

- `LLAMA_EMBED_BIN`: path to `llama-embedding` (defaults to the binary on PATH)
- `LLAMA_EMBED_MODEL`: path to your GGUF model (used if `--embedding-model` is not provided)

LLM suggestions remain optional: pass `--enable-llm` with `--llm-model /path/to/model.gguf` if you also want the LLM-based generator.

## 🧠 Architecture

Ghosttype uses an ensemble of models:

* `markov`: Lightweight transition-based predictor
* `freq`: Frequency-based suggestion engine
* `alias`: Shell aliases from `.zshrc`/`.bashrc`
* `context`: Targets from `Makefile`, `package.json`, `pom.xml`, etc.
* `embedding`: Vector search powered by `llama-embedding` (llama.cpp)

All models implement a unified `SuggestModel` interface and are combined via `ensemble.Model`.

## 🗂 Project Structure

```
.
├── rust/             # Rust CLI implementation (primary)
├── script/           # Helper scripts (install, etc.)
├── testdata/         # Fixtures
└── README.md
```


## 📜 License

Apache-2.0
See [LICENSE](./LICENSE) for full terms.
