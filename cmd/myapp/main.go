package main

import (
	"errors"
	"log/slog"

	"github.com/RouzieZ/THD/pkg/logger"
	"github.com/RouzieZ/THD/pkg/storage"
)

func main() {
	log := logger.New(slog.LevelInfo)
	engine := storage.New(log)

	if err := engine.Set("сообщение", []byte("привет")); err != nil {
		log.Error("не могу сохранить значение", "ошибка", err)
		return
	}

	value, err := engine.Get("сообщение")
	if err != nil {
		log.Error("нечитаемое значение", "ошибка", err)
		return
	}
	log.Info("читаемое значение", "значение", string(value), "ключи", engine.Len())

	if err := engine.Delete("сообщение"); err != nil {
		log.Error("не могу удалить значение", "ошибка", err)
		return
	}

	if _, err := engine.Get("сообщение"); errors.Is(err, storage.ErrNotFound) {
		log.Info("ключ отсутствует", "ошибка", err)
	}
}
