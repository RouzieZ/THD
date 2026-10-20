package storage_test

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/RouzieZ/THD/pkg/storage"
)

// mockLogHandler заменяет вывод логов сбором записей в памяти.
// Производные обработчики разделяют записи, сохраняя собственные атрибуты и группы.
type mockLogHandler struct {
	state  *mockLogState
	attrs  []slog.Attr
	groups []string
}

type mockLogState struct {
	mu      sync.Mutex
	records []slog.Record
}

var _ slog.Handler = (*mockLogHandler)(nil)

func newEngineWithMock() (*storage.Engine, *mockLogHandler) {
	handler := &mockLogHandler{state: &mockLogState{}}
	return storage.New(slog.New(handler)), handler
}

func (h *mockLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *mockLogHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	entry := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	entry.AddAttrs(h.attrs...)
	entry.AddAttrs(groupAttrs(h.groups, attrs)...)

	h.state.mu.Lock()
	h.state.records = append(h.state.records, entry)
	h.state.mu.Unlock()
	return nil
}

func (h *mockLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(slices.Clone(h.attrs), groupAttrs(h.groups, attrs)...)
	return &clone
}

func (h *mockLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(slices.Clone(h.groups), name)
	return &clone
}

func (h *mockLogHandler) Records() []slog.Record {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	records := make([]slog.Record, len(h.state.records))
	for i, record := range h.state.records {
		records[i] = record.Clone()
	}
	return records
}

func groupAttrs(groups []string, attrs []slog.Attr) []slog.Attr {
	for i := len(groups) - 1; i >= 0; i-- {
		attrs = []slog.Attr{{Key: groups[i], Value: slog.GroupValue(attrs...)}}
	}
	return attrs
}
