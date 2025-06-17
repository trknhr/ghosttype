package worker

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/store"
)

func LaunchWorker(historyStore store.HistoryStore, historyLoader history.HistoryLoader) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := RunHistoryWorker(ctx, historyStore, historyLoader); err != nil {
			logger.Error("background learning failed: %v", err)
		}
	}()
}

func RunHistoryWorker(ctx context.Context, historyStore store.HistoryStore, historyLoader history.HistoryLoader) error {
	// Retrieve last processed mtime (to detect if update is needed)
	lastMtime, err := historyStore.GetLastProcessedMtime(historyLoader.Key(), historyLoader.Path())
	if err != nil {
		return err
	}

	currentMtime, err := historyLoader.GetCurrentMtime()

	if err != nil {
		return err
	}

	// Skip if history file hasn't changed
	if currentMtime <= lastMtime {
		logger.Debug("history file not modified since last processing")
		return nil
	}

	historyEntries, err := historyLoader.LoadCommands()
	if err != nil {
		return err
	}

	var cleaned []string
	for _, entry := range historyEntries {
		s := strings.TrimSpace(entry)
		if s != "" && utf8.ValidString(s) {
			cleaned = append(cleaned, s)
		}
	}

	err = historyStore.SaveHistory(cleaned)
	if err != nil {
		return err
	}

	return historyStore.UpdateMetadata(historyLoader.Key(), historyLoader.Path(), currentMtime)
}
