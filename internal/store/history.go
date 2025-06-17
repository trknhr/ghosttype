package store

import (
	"database/sql"
	"strings"

	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/utils"
)

type HistoryStore interface {
	SaveHistory(entries []string) error
	GetLastProcessedMtime(key, path string) (int64, error)
	UpdateMetadata(key, path string, mtime int64) error
}

type SQLHistoryStore struct {
	db *sql.DB
}

func NewSQLHistoryStore(db *sql.DB) HistoryStore {
	return &SQLHistoryStore{db: db}
}

func (s *SQLHistoryStore) SaveHistory(entries []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        INSERT INTO history(command, hash, count)
        VALUES (?, ?, 1)
        ON CONFLICT(hash) DO UPDATE SET count = count + 1
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cmd := range entries {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		hash := utils.Hash(cmd)
		if _, err := stmt.Exec(cmd, hash); err != nil {
			logger.Error("failed to insert command: %s, %v", cmd, err)
		}
	}
	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit history tx: %v", err)
		return err
	}
	return nil
}

func (s *SQLHistoryStore) GetLastProcessedMtime(key, path string) (int64, error) {
	var mtime int64
	err := s.db.QueryRow("SELECT mtime FROM meta WHERE key = ? AND path = ?", key, path).Scan(&mtime)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return mtime, err
}

func (s *SQLHistoryStore) UpdateMetadata(key, path string, mtime int64) error {
	_, err := s.db.Exec(`
        INSERT INTO meta (key, path, mtime) 
        VALUES (?, ?, ?) 
        ON CONFLICT(key) DO UPDATE SET 
            path = excluded.path,
            mtime = excluded.mtime`,
		key, path, mtime)
	return err
}
