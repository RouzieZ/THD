// Пакет storage реализует хранилище ключ–значение в оперативной памяти.
package storage

import (
	"bytes"
	"errors"
	"log/slog"
	"sync"

	"github.com/RouzieZ/THD/pkg/logger"
)

var (
	// ErrEmptyKey возвращается при передаче пустого ключа.
	ErrEmptyKey = errors.New("storage: key must not be empty")
	// ErrNotFound возвращается, если ключ отсутствует в хранилище.
	ErrNotFound = errors.New("storage: key not found")
)

// Engine хранит значения в памяти и записывает операции в журнал. Создаётся через New.
// Методы безопасны для одновременного вызова из нескольких горутин.
// После создания Engine нельзя копировать.
type Engine struct {
	mu     sync.RWMutex
	items  map[string][]byte
	logger *slog.Logger
}

// New создаёт пустое хранилище. Если log равен nil, JSON-логи выводятся в stdout
// с минимальным уровнем INFO.
func New(log *slog.Logger) *Engine {
	if log == nil {
		log = logger.New(slog.LevelInfo)
	}

	return &Engine{
		items:  make(map[string][]byte),
		logger: log.With("component", "storage"),
	}
}

// Set добавляет или перезаписывает значение, сохраняя собственную копию байтов.
// Значения nil и пустые срезы допустимы; пустой ключ приводит к ошибке ErrEmptyKey.
func (e *Engine) Set(key string, value []byte) error {
	if key == "" {
		e.logOperation("set", key, 0, ErrEmptyKey)
		return ErrEmptyKey
	}

	value = bytes.Clone(value)
	e.mu.Lock()
	e.items[key] = value
	e.mu.Unlock()

	e.logOperation("set", key, len(value), nil)
	return nil
}

// Get возвращает копию значения или ошибку ErrNotFound, если ключ отсутствует.
// Изменение полученного среза байтов не влияет на данные в хранилище.
func (e *Engine) Get(key string) ([]byte, error) {
	if key == "" {
		e.logOperation("get", key, 0, ErrEmptyKey)
		return nil, ErrEmptyKey
	}

	e.mu.RLock()
	value, exists := e.items[key]
	value = bytes.Clone(value)
	e.mu.RUnlock()

	if !exists {
		e.logOperation("get", key, 0, ErrNotFound)
		return nil, ErrNotFound
	}

	e.logOperation("get", key, len(value), nil)
	return value, nil
}

// Delete удаляет значение или возвращает ошибку ErrNotFound, если ключ отсутствует.
func (e *Engine) Delete(key string) error {
	if key == "" {
		e.logOperation("delete", key, 0, ErrEmptyKey)
		return ErrEmptyKey
	}

	e.mu.Lock()
	value, exists := e.items[key]
	delete(e.items, key)
	e.mu.Unlock()

	if !exists {
		e.logOperation("delete", key, 0, ErrNotFound)
		return ErrNotFound
	}

	e.logOperation("delete", key, len(value), nil)
	return nil
}

// Len возвращает текущее количество ключей в хранилище.
func (e *Engine) Len() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.items)
}

// logOperation записывает результат операции в журнал без содержимого значения.
func (e *Engine) logOperation(operation, key string, size int, err error) {
	attrs := []any{
		slog.String("operation", operation),
		slog.String("key", key),
	}
	if err != nil {
		e.logger.Warn("storage operation failed", append(attrs, slog.Any("error", err))...)
		return
	}

	e.logger.Info("storage operation completed", append(attrs, slog.Int("bytes", size))...)
}
