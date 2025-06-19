package history

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
)

func TestZshHistoryLoader_GetCurrentMtime(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "zsh_history_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	loader := &ZshHistoryLoader{path: tmpfile.Name()}

	mtime, err := loader.GetCurrentMtime()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	stat, _ := os.Stat(tmpfile.Name())
	if mtime != stat.ModTime().Unix() {
		t.Errorf("mtime mismatch: got %v, want %v", mtime, stat.ModTime().Unix())
	}
}

func TestHistoryLoader_Mock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockHistoryLoader(ctrl)

	mock.EXPECT().
		LoadTail(5).
		Return([]string{"ls -la", "git status"}, nil)

	mock.EXPECT().
		GetCurrentMtime().
		Return(int64(123456789), nil)

	mock.EXPECT().
		Path().
		Return("/mock/path")

	mock.EXPECT().
		Key().
		Return("zsh_history")

	cmds, err := mock.LoadTail(5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(cmds) != 2 || cmds[0] != "ls -la" {
		t.Errorf("unexpected LoadTail result: %+v", cmds)
	}

	mtime, err := mock.GetCurrentMtime()
	if err != nil || mtime != 123456789 {
		t.Errorf("unexpected GetCurrentMtime: got %v %v", mtime, err)
	}

	path := mock.Path()
	if path != "/mock/path" {
		t.Errorf("unexpected Path: %v", path)
	}

	if mock.Key() != "zsh_history" {
		t.Errorf("unexpected Key: %v", mock.Key())
	}
}

// BashHistoryLoaderのnot implementedテスト
func TestBashHistoryLoader_NotImplemented(t *testing.T) {
	loader := &BashHistoryLoader{path: "/dev/null"}
	assertPanic := func(f func()) (didPanic bool) {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		f()
		return
	}
	if !assertPanic(func() { loader.LoadTail(1) }) {
		t.Error("expected panic on LoadTail for BashHistoryLoader")
	}
	if !assertPanic(func() { loader.LoadCommands() }) {
		t.Error("expected panic on LoadCommands for BashHistoryLoader")
	}
}
