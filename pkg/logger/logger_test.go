package logger_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/RouzieZ/THD/pkg/logger"
)

func TestNewJSONOutputAndLevel(t *testing.T) {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	for i, minimum := range levels {
		t.Run(minimum.String(), func(t *testing.T) {
			output := captureStdout(t, func() {
				log := logger.New(minimum)
				for _, level := range levels {
					log.Log(t.Context(), level, "operation", "key", "example")
				}
			})

			lines := strings.Split(strings.TrimSpace(output), "\n")
			if got, want := len(lines), len(levels)-i; got != want {
				t.Fatalf("строк в stdout: %d, ожидалось %d; вывод: %q", got, want, output)
			}
			for j, line := range lines {
				var entry struct {
					Time    string `json:"time"`
					Level   string `json:"level"`
					Message string `json:"msg"`
					Key     string `json:"key"`
				}
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("невалидный JSON: %v; строка: %q", err, line)
				}
				if _, err := time.Parse(time.RFC3339Nano, entry.Time); err != nil {
					t.Errorf("некорректное время в логе: %q", entry.Time)
				}
				if want := levels[i+j].String(); entry.Level != want {
					t.Errorf("уровень = %q, ожидалось %q", entry.Level, want)
				}
				if entry.Message != "operation" || entry.Key != "example" {
					t.Errorf("сообщение или атрибуты потеряны: %+v", entry)
				}
			}
		})
	}
}

// captureStdout использует канал ОС без записи на диск.
// Вызывающие тесты не должны выполняться параллельно: os.Stdout общий для процесса.
func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("не удалось создать канал stdout: %v", err)
	}
	original := os.Stdout
	t.Cleanup(func() {
		os.Stdout = original
		reader.Close()
		writer.Close()
	})

	os.Stdout = writer
	run()
	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatalf("не удалось закрыть канал stdout: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("не удалось прочитать stdout: %v", err)
	}
	return string(output)
}
