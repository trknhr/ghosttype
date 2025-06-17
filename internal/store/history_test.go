package store_test

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/trknhr/ghosttype/internal/store"
)

func TestHistoryStore_SaveHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := store.NewMockHistoryStore(ctrl)

	// return nil for SaveHistory call
	mock.EXPECT().
		SaveHistory([]string{"ls -la", "git status"}).
		Return(nil)

	err := mock.SaveHistory([]string{"ls -la", "git status"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHistoryStore_GetLastProcessedMtime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := store.NewMockHistoryStore(ctrl)

	mock.EXPECT().
		GetLastProcessedMtime("zsh_history", "/tmp/hist").
		Return(int64(12345), nil)

	mtime, err := mock.GetLastProcessedMtime("zsh_history", "/tmp/hist")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if mtime != 12345 {
		t.Errorf("expected 12345, got %d", mtime)
	}
}

func TestHistoryStore_UpdateMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := store.NewMockHistoryStore(ctrl)

	mock.EXPECT().
		UpdateMetadata("zsh_history", "/tmp/hist", int64(99999)).
		Return(nil)

	err := mock.UpdateMetadata("zsh_history", "/tmp/hist", 99999)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// Error Case Tests
func TestHistoryStore_SaveHistory_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := store.NewMockHistoryStore(ctrl)

	mock.EXPECT().
		SaveHistory(gomock.Any()).
		Return(errors.New("mock error"))

	err := mock.SaveHistory([]string{"badcommand"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
