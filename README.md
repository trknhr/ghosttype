# markov-cli

**markov-cli** is a terminal-based command suggestion tool powered by a Markov chain model.  
It learns your shell history and helps you autocomplete commands in an interactive TUI using the [Bubbletea](https://github.com/charmbracelet/bubbletea) framework.

---

## 🚧 Status: Under Development

This project is still in an early development phase.  
**Functionality may be unstable or incomplete. Use at your own risk.**

Your feedback and contributions are welcome!

---

## ✨ Features

- 🧠 Learns from your `.zsh_history` or `.bash_history`
- ⚡ Token-based Markov model for smarter suggestions (e.g., `npm` → `run`, `install`, etc.)
- 🎯 Real-time suggestions as you type
- ⌨️ Interactive CLI with up/down navigation and selection
- 🧼 Clears the screen and outputs the final command on exit

---

## 📸 Demo

```
$ markov-cli
Command: npm

Suggestions:
  install
> run
  test

# After pressing Enter:
npm run
```

---

## 🛠 Installation

```bash
git clone https://github.com/trknhr/markov-cli.git
cd markov-cli
go build -o markov-cli
./markov-cli
```

---

## 📂 Directory Structure

```
.
├── history/       # History parsers for bash and zsh
├── marcov/        # Token-level Markov model implementation
├── ui/            # Bubbletea-based TUI
├── main.go        # Entry point
├── go.mod
└── README.md
```

---

## 🔍 How It Works

- History is loaded from `~/.zsh_history` by default
- Each line is tokenized (e.g., `git commit -m` → `["git", "commit", "-m"]`)
- Transitions between tokens are counted and stored as a Markov chain
- As you type, the last token is used to predict the most likely next token(s)
- Suggestions are updated dynamically

---

## ⌨️ Keyboard Shortcuts

- ↑ / ↓ : Navigate suggestions
- `Enter` : Add selected token to input and quit
- `q` or `Ctrl+C` : Quit without output

---

## 🧪 Optional Enhancements (Planned)

- Multi-step interaction (keep suggesting until the user confirms)
- Support for persistent model across sessions
- Improved UI and fuzzy matching
- Better integration with shells (`zle`, `eval`, etc.)

---

## 📝 License

Apache-2.0  
See [LICENSE](./LICENSE) for full terms.
