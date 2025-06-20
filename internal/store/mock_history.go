// Code generated by MockGen. DO NOT EDIT.
// Source: internal/store/history.go

// Package store is a generated GoMock package.
package store

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockHistoryStore is a mock of HistoryStore interface.
type MockHistoryStore struct {
	ctrl     *gomock.Controller
	recorder *MockHistoryStoreMockRecorder
}

// MockHistoryStoreMockRecorder is the mock recorder for MockHistoryStore.
type MockHistoryStoreMockRecorder struct {
	mock *MockHistoryStore
}

// NewMockHistoryStore creates a new mock instance.
func NewMockHistoryStore(ctrl *gomock.Controller) *MockHistoryStore {
	mock := &MockHistoryStore{ctrl: ctrl}
	mock.recorder = &MockHistoryStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHistoryStore) EXPECT() *MockHistoryStoreMockRecorder {
	return m.recorder
}

// GetLastProcessedMtime mocks base method.
func (m *MockHistoryStore) GetLastProcessedMtime(key, path string) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLastProcessedMtime", key, path)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastProcessedMtime indicates an expected call of GetLastProcessedMtime.
func (mr *MockHistoryStoreMockRecorder) GetLastProcessedMtime(key, path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastProcessedMtime", reflect.TypeOf((*MockHistoryStore)(nil).GetLastProcessedMtime), key, path)
}

// SaveHistory mocks base method.
func (m *MockHistoryStore) SaveHistory(entries []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveHistory", entries)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveHistory indicates an expected call of SaveHistory.
func (mr *MockHistoryStoreMockRecorder) SaveHistory(entries interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveHistory", reflect.TypeOf((*MockHistoryStore)(nil).SaveHistory), entries)
}

// UpdateMetadata mocks base method.
func (m *MockHistoryStore) UpdateMetadata(key, path string, mtime int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateMetadata", key, path, mtime)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateMetadata indicates an expected call of UpdateMetadata.
func (mr *MockHistoryStoreMockRecorder) UpdateMetadata(key, path, mtime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMetadata", reflect.TypeOf((*MockHistoryStore)(nil).UpdateMetadata), key, path, mtime)
}
