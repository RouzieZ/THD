# THD — Storage engine

Потокобезопасное хранилище ключ–значение на Go. Данные хранятся в оперативной
памяти и теряются при завершении процесса. Модуль журналирования пишет JSON-логи
операций в stdout. Проект использует только стандартную библиотеку Go.

## Структура

```text
cmd/
  myapp/main.go         — исходная заготовка точки входа
configs/
  config.yaml          — заготовка конфигурации
internal/
  app/app.go
  config/config.go
  handlers/handlers.go
  repository/repo.go
  services/service.go
pkg/
  publiclib/publiclib.go — заготовка общего пакета
  storage/
    engine.go           — хранение данных и операции над ними
    engine_test.go      — проверки хранилища и журналирования
    mock_logger_test.go — мок slog.Handler со сбором записей в памяти
  logger/
    logger.go           — настройка структурированных логов через log/slog
    logger_test.go      — проверки JSON-вывода и уровней логирования
Dockerfile              — заготовка контейнера
go.mod
go.sum
README.md
```

Исходные заготовки проекта сохранены в `cmd`, `configs`, `internal` и
`pkg/publiclib`. Хранилище и модуль журналирования реализованы в `pkg/storage`
и `pkg/logger`; пример их использования приведён ниже.

## API

| Метод | Поведение |
| --- | --- |
| `storage.New(log *slog.Logger) *Engine` | Создаёт пустое хранилище. При `nil` включает JSON-логи в stdout с уровнем INFO. |
| `Set(key string, value []byte) error` | Добавляет или перезаписывает значение. |
| `Get(key string) ([]byte, error)` | Возвращает копию значения; для отсутствующего ключа — `ErrNotFound`. |
| `Delete(key string) error` | Удаляет значение; для отсутствующего ключа — `ErrNotFound`. |
| `Len() int` | Возвращает количество ключей. |

Пустой ключ возвращает `ErrEmptyKey`; ключи обрабатываются без изменения пробелов
и регистра. Значения `nil` и пустые байтовые срезы допустимы. Хранилище копирует
байты при записи и чтении, поэтому изменение переданного или полученного среза
не меняет сохранённые данные. Методы можно вызывать из нескольких горутин;
сам `Engine` после создания копировать нельзя.

## Использование

```go
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

    if err := engine.Set("greeting", []byte("hello")); err != nil {
        log.Error("cannot store value", "error", err)
        return
    }

    value, err := engine.Get("greeting")
    if err != nil {
        log.Error("cannot read value", "error", err)
        return
    }
    log.Info("value read", "value", string(value), "keys", engine.Len())

    if err := engine.Delete("greeting"); err != nil {
        log.Error("cannot delete value", "error", err)
        return
    }

    if _, err := engine.Get("greeting"); errors.Is(err, storage.ErrNotFound) {
        log.Info("key is absent")
    }
}
```

## Журналирование

Каждый вызов `Set`, `Get` и `Delete` создаёт запись с операцией и ключом.
Успешные операции имеют уровень INFO и поле `bytes` с размером значения;
неуспешные — уровень WARN и поле `error`. Значения хранилища в журнал не попадают.
Уровень фильтрации задаётся при создании логгера. Пример записи:

```json
{"time":"2026-10-20T12:00:00+03:00","level":"INFO","msg":"storage operation completed","component":"storage","operation":"set","key":"greeting","bytes":5}
```

Логи служат для наблюдения за операциями; при параллельных вызовах порядок строк
может отличаться от порядка изменения данных. Записи не сохраняются движком
на диск и не используются для восстановления данных.

## Сборка

Нужен Go версии, указанной в `go.mod`, или новее.

```sh
go build ./...
go vet ./...
```

## Тесты

```sh
go test ./...
go test -race -cover ./...
```

Тесты проверяют чтение, запись, перезапись, удаление, ошибки, точное сохранение
ключей, копирование байтов и одновременную работу из нескольких горутин.
Мок `slog.Handler` собирает записи в памяти: проверяются уровень, операция, ключ,
размер значения и ошибка, а также отсутствие содержимого значений в логах.
Отдельные тесты логгера перехватывают stdout через канал ОС и проверяют формат
JSON и фильтрацию уровней сообщений. Внешние библиотеки для моков не нужны.
