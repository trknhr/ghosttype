package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var (
	logger *slog.Logger
	level  = slog.LevelInfo
)

func Init(logfilePath string, levelStr string) error {
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "none":
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		logger = slog.New(slog.NewTextHandler(devNull, nil))
		return nil
	default:
		level = slog.LevelInfo
	}

	var opts = &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if logfilePath != "" {
		dir := filepath.Dir(logfilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		handler = slog.NewTextHandler(io.MultiWriter(os.Stderr, f), opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger = slog.New(handler)
	return nil
}

func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}
func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}
func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}
func Error(msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}
