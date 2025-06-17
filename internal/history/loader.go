package history

import (
	"os"
	"path/filepath"
)

func DefaultHistoryPath(shell string) string {
	home := os.Getenv("HOME")
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zsh_history")
	case "bash":
		return filepath.Join(home, ".bash_history")
	case "fish":
		return filepath.Join(home, ".local", "share", "fish", "fish_history")
	case "powershell":
		return filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadline", "ConsoleHost_history.txt")
	default:
		return ""
	}
}

func NewHistoryLoaderAuto() HistoryLoader {
	shell := DetectShell()
	path := DefaultHistoryPath(shell)
	switch shell {
	case "zsh":
		return &ZshHistoryLoader{path: path}
	case "bash":
		return &BashHistoryLoader{path: path}
	default:
		// fallback: zsh
		return &ZshHistoryLoader{path: path}
	}
}

type HistoryLoader interface {
	LoadTail(n int) ([]string, error)
	LoadCommands() ([]string, error)
	GetCurrentMtime() (int64, error)
	Path() string
	Key() string
}

type ZshHistoryLoader struct {
	path string
}

func (z *ZshHistoryLoader) LoadTail(n int) ([]string, error) {
	return LoadZshHistoryTail(z.path, n)
}
func (z *ZshHistoryLoader) LoadCommands() ([]string, error) {
	return LoadZshHistoryCommands(z.path)
}

func (z *ZshHistoryLoader) GetCurrentMtime() (int64, error) {
	info, err := os.Stat(z.path)
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}

func (z *ZshHistoryLoader) Path() string {
	return z.path
}

func (z *ZshHistoryLoader) Key() string {
	return "zsh_history"
}

type BashHistoryLoader struct {
	path string
}

func (b *BashHistoryLoader) LoadTail(n int) ([]string, error) {
	panic("This method is not implemented for BashHistoryLoader")
}
func (b *BashHistoryLoader) LoadCommands() ([]string, error) {
	panic("This method is not implemented for LoadCommands in BashHistoryLoader")
}

func (z *BashHistoryLoader) GetCurrentMtime() (int64, error) {
	info, err := os.Stat(z.path)
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}

func (z *BashHistoryLoader) Path() string {
	return z.path
}

func (z *BashHistoryLoader) Key() string {
	return "bash_history"
}
