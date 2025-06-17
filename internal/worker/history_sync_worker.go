package worker

import (
	"strings"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/store"
)

type HistorySyncWorker struct {
	store  store.HistoryStore
	loader history.HistoryLoader
}

func NewHistorySyncWorker(store store.HistoryStore, loader history.HistoryLoader) *HistorySyncWorker {
	return &HistorySyncWorker{store, loader}
}

func (h *HistorySyncWorker) Key() string  { return h.loader.Key() }
func (h *HistorySyncWorker) Path() string { return h.loader.Path() }
func (h *HistorySyncWorker) NeedsReload() bool {
	last, err := h.store.GetLastProcessedMtime(h.Key(), h.Path())
	if err != nil {
		return true // conservative: try to reload if error
	}
	curr, err := h.loader.GetCurrentMtime()
	if err != nil {
		return false // don't try if can't stat
	}
	return curr > last
}

func (h *HistorySyncWorker) Sync() error {
	historyEntries, err := h.loader.LoadCommands()
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
	if err := h.store.SaveHistory(cleaned); err != nil {
		return err
	}
	curr, err := h.loader.GetCurrentMtime()
	if err != nil {
		return err
	}
	return h.store.UpdateMetadata(h.Key(), h.Path(), curr)
}
