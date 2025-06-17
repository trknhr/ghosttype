package logger

import (
	"fmt"
	"os"
	"sync"
)

var once sync.Once

func WarnOnce() {
	once.Do(func() {
		fmt.Fprintln(os.Stderr, "⚠️ Unable to connect to OLLAMA. Using only local models.")
		fmt.Fprintln(os.Stderr, "🧠 To enable OLLAMA-based suggestions, please run `ollama serve`. Learn more at https://github.com/trknhr/ghosttype#enable-llm-powered-suggestions-via-ollama")
	})
}
