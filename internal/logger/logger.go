package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	NONE
)

var (
	level     = INFO
	stdLogger = log.New(os.Stderr, "[ghosttype] ", log.LstdFlags)
)

func Init(logfilePath string, levelStr string) error {
	// ログレベル設定
	switch strings.ToLower(levelStr) {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARN
	case "error":
		level = ERROR
	case "none":
		level = NONE
	default:
		level = INFO
	}

	// outputFile
	if logfilePath != "" {
		dir := filepath.Dir(logfilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		stdLogger.SetOutput(io.MultiWriter(os.Stderr, f)) // 複数出力可
	} else {
		stdLogger.SetOutput(os.Stderr)
	}
	return nil
}

func Debug(msg string, args ...any) {
	if level <= DEBUG {
		stdLogger.Printf("[DEBUG] "+msg, args...)
	}
}
func Info(msg string, args ...any) {
	if level <= INFO {
		stdLogger.Printf("[INFO] "+msg, args...)
	}
}
func Warn(msg string, args ...any) {
	if level <= WARN {
		stdLogger.Printf("[WARN] "+msg, args...)
	}
}
func Error(msg string, args ...any) {
	if level <= ERROR {
		stdLogger.Printf("[ERROR] "+msg, args...)
	}
}
