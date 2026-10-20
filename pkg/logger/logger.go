// Пакет logger настраивает вывод структурированных журналов операций в stdout.
package logger

import (
	"log/slog"
	"os"
)

// New создаёт JSON-логгер с указанным минимальным уровнем сообщений.
// Логгер безопасен для одновременного использования из нескольких горутин.
func New(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
