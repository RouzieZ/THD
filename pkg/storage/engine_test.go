package storage_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/RouzieZ/THD/pkg/storage"
)

func TestNew(t *testing.T) {
	t.Parallel()

	engine := storage.New(nil)
	if got := engine.Len(); got != 0 {
		t.Fatalf("новое хранилище содержит %d ключей, ожидалось 0", got)
	}
}

func TestEngineSetGet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value []byte
	}{
		{name: "text", value: []byte("hello")},
		{name: "binary", value: []byte{0, 1, 0xff, 0}},
		{name: "nil", value: nil},
		{name: "empty", value: []byte{}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine, _ := newEngineWithMock()
			if err := engine.Set("key", test.value); err != nil {
				t.Fatalf("Set() вернул ошибку: %v", err)
			}
			got, err := engine.Get("key")
			if err != nil {
				t.Fatalf("Get() вернул ошибку: %v", err)
			}
			if !reflect.DeepEqual(got, test.value) {
				t.Errorf("Get() = %#v, ожидалось %#v", got, test.value)
			}
			if got := engine.Len(); got != 1 {
				t.Errorf("Len() = %d, ожидалось 1", got)
			}
		})
	}
}

func TestEngineOverwrite(t *testing.T) {
	t.Parallel()
	engine, _ := newEngineWithMock()

	for _, value := range []string{"old", "new value"} {
		if err := engine.Set("key", []byte(value)); err != nil {
			t.Fatalf("Set() вернул ошибку: %v", err)
		}
	}
	got, err := engine.Get("key")
	if err != nil {
		t.Fatalf("Get() вернул ошибку: %v", err)
	}
	if string(got) != "new value" {
		t.Errorf("Get() = %q, ожидалось %q", got, "new value")
	}
	if got := engine.Len(); got != 1 {
		t.Errorf("после перезаписи Len() = %d, ожидалось 1", got)
	}
}

func TestEngineCopiesValues(t *testing.T) {
	t.Parallel()
	engine, _ := newEngineWithMock()

	input := []byte("original")
	if err := engine.Set("key", input); err != nil {
		t.Fatalf("Set() вернул ошибку: %v", err)
	}
	input[0] = 'X'

	got, err := engine.Get("key")
	if err != nil {
		t.Fatalf("Get() вернул ошибку: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("изменение входного среза повлияло на хранилище: %q", got)
	}
	got[0] = 'Y'

	stored, err := engine.Get("key")
	if err != nil {
		t.Fatalf("повторный Get() вернул ошибку: %v", err)
	}
	if string(stored) != "original" {
		t.Errorf("изменение возвращённого среза повлияло на хранилище: %q", stored)
	}
}

func TestEngineKeysAreExact(t *testing.T) {
	t.Parallel()
	engine, _ := newEngineWithMock()
	keys := []string{"key", "KEY", " key "}

	for _, key := range keys {
		if err := engine.Set(key, []byte(key)); err != nil {
			t.Fatalf("Set(%q) вернул ошибку: %v", key, err)
		}
	}
	for _, key := range keys {
		got, err := engine.Get(key)
		if err != nil || string(got) != key {
			t.Errorf("Get(%q) = (%q, %v), ожидалось (%q, nil)", key, got, err, key)
		}
	}
	if got := engine.Len(); got != len(keys) {
		t.Errorf("Len() = %d, ожидалось %d", got, len(keys))
	}
}

func TestEngineDelete(t *testing.T) {
	t.Parallel()

	for _, value := range [][]byte{[]byte("value"), nil, {}} {
		t.Run(fmt.Sprintf("%#v", value), func(t *testing.T) {
			t.Parallel()
			engine, _ := newEngineWithMock()
			if err := engine.Set("key", value); err != nil {
				t.Fatalf("Set() вернул ошибку: %v", err)
			}
			if err := engine.Delete("key"); err != nil {
				t.Fatalf("Delete() вернул ошибку: %v", err)
			}
			if got := engine.Len(); got != 0 {
				t.Errorf("после удаления Len() = %d, ожидалось 0", got)
			}
			got, err := engine.Get("key")
			if got != nil || !errors.Is(err, storage.ErrNotFound) {
				t.Errorf("Get() после удаления = (%v, %v), ожидалось (nil, ErrNotFound)", got, err)
			}
		})
	}
}

func TestEngineOperationsAndLogs(t *testing.T) {
	t.Parallel()
	engine, handler := newEngineWithMock()
	const value = "private value that must not appear in logs"
	get := func(key string) error {
		_, err := engine.Get(key)
		return err
	}

	cases := []struct {
		name      string
		operation string
		key       string
		run       func() error
		wantErr   error
		wantLen   int
	}{
		{"set", "set", "key", func() error { return engine.Set("key", []byte(value)) }, nil, 1},
		{"get", "get", "key", func() error { return get("key") }, nil, 1},
		{"set_empty_key", "set", "", func() error { return engine.Set("", []byte(value)) }, storage.ErrEmptyKey, 1},
		{"get_empty_key", "get", "", func() error { return get("") }, storage.ErrEmptyKey, 1},
		{"delete_empty_key", "delete", "", func() error { return engine.Delete("") }, storage.ErrEmptyKey, 1},
		{"get_missing", "get", "missing", func() error { return get("missing") }, storage.ErrNotFound, 1},
		{"delete_missing", "delete", "missing", func() error { return engine.Delete("missing") }, storage.ErrNotFound, 1},
		{"delete", "delete", "key", func() error { return engine.Delete("key") }, nil, 0},
		{"get_deleted", "get", "key", func() error { return get("key") }, storage.ErrNotFound, 0},
		{"delete_again", "delete", "key", func() error { return engine.Delete("key") }, storage.ErrNotFound, 0},
	}

	// Порядок важен: операции выполняются над одним хранилищем.
	for i, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, test.wantErr) {
				t.Fatalf("ошибка операции = %v, ожидалось %v", err, test.wantErr)
			}
			if got := engine.Len(); got != test.wantLen {
				t.Errorf("Len() = %d, ожидалось %d", got, test.wantLen)
			}

			records := handler.Records()
			if len(records) != i+1 {
				t.Fatalf("записей в журнале: %d, ожидалось %d", len(records), i+1)
			}
			record := records[i]
			attrs := make(map[string]any)
			record.Attrs(func(attr slog.Attr) bool {
				attrs[attr.Key] = attr.Value.Resolve().Any()
				return true
			})
			wantAttrs := map[string]any{
				"component": "storage",
				"operation": test.operation,
				"key":       test.key,
			}
			wantLevel := slog.LevelInfo
			if test.wantErr != nil {
				wantAttrs["error"] = test.wantErr
				wantLevel = slog.LevelWarn
			} else {
				wantAttrs["bytes"] = int64(len(value))
			}
			if record.Level != wantLevel {
				t.Errorf("уровень лога = %v, ожидалось %v", record.Level, wantLevel)
			}
			if !reflect.DeepEqual(attrs, wantAttrs) {
				t.Errorf("атрибуты лога = %#v, ожидалось %#v", attrs, wantAttrs)
			}
			if strings.Contains(record.Message, value) {
				t.Error("содержимое значения попало в сообщение журнала")
			}
		})
	}
}

func TestEngineConcurrentAccess(t *testing.T) {
	t.Parallel()
	engine, handler := newEngineWithMock()
	const workers, iterations = 16, 32
	var wg sync.WaitGroup

	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("worker-%d", worker)
			for iteration := range iterations {
				value := []byte(fmt.Sprintf("value-%d", iteration))
				if err := engine.Set(key, value); err != nil {
					t.Errorf("Set(%q) вернул ошибку: %v", key, err)
					return
				}
				got, err := engine.Get(key)
				if err != nil || !bytes.Equal(got, value) {
					t.Errorf("Get(%q) = (%q, %v), ожидалось (%q, nil)", key, got, err, value)
					return
				}
				if got := engine.Len(); got < 1 || got > workers {
					t.Errorf("количество ключей вне допустимого диапазона: %d", got)
				}
				if err := engine.Delete(key); err != nil {
					t.Errorf("Delete(%q) вернул ошибку: %v", key, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := engine.Len(); got != 0 {
		t.Errorf("после завершения горутин Len() = %d, ожидалось 0", got)
	}
	if got, want := len(handler.Records()), workers*iterations*3; got != want {
		t.Errorf("записей в журнале: %d, ожидалось %d", got, want)
	}
}
