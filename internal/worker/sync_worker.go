package worker

import "github.com/trknhr/ghosttype/internal/logger"

type SyncWorker interface {
	Key() string
	Path() string
	NeedsReload() bool
	Sync() error
}

func LaunchSyncWorkers(syncers ...SyncWorker) {
	for _, s := range syncers {
		go func(s SyncWorker) {
			if !s.NeedsReload() {
				logger.Debug("[%s] sync skipped (up-to-date)", s.Key())
				return
			}
			if err := s.Sync(); err != nil {
				logger.Error("[%s] sync failed: %v", s.Key(), err)
			} else {
				logger.Info("[%s] sync done", s.Key())
			}
		}(s)
	}
}
