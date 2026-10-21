# SFDDB HTTP: каркас проекта

Этот проект показывает только HTTP-границу системы из `HLD_SFDDB.drawio`.
Она построена на стандартном пакете Go `net/http`. Все остальные узлы схемы
представлены моками: они позволяют проверить связи, но не гарантируют
транзакционность, долговечность данных или настоящую репликацию.

## Структура

```text
sfddb-http/
├── go.mod
├── cmd/sfddb/main.go                 # создание объектов и запуск
├── internal/httpserver/server.go    # HTTP-сервер, Config и callback-хэндлеры
├── internal/httpserver/server_test.go
└── internal/mock/components.go      # моки компонентов HLD
```

## Связи с диаграммой

```text
клиент фитнес-клуба -> ServiceClient (mock) -> HTTP server
Ops -> AdminCLI (mock) -> HTTP server
HTTP server -> Translator.Parse (callback) -> Engine (mock)
Engine -> Transaction / ACID (mock) -> WAL (mock) -> Disk (mock)
                                           └── Y / network / replica (mock)
```

`GET` читает значение из памяти мока Engine. `SET` и `DELETE` проходят через
моки Transaction, WAL, Disk и Replicator. После остановки процесса данные
теряются: настоящий WAL пока отсутствует.

## Запуск

Из корня проекта:

```bash
go run ./cmd/sfddb
```

Для другого адреса: `HTTP_ADDR=127.0.0.1:9090 go run ./cmd/sfddb`.

Проверка в другом терминале:

```bash
curl http://127.0.0.1:8081/healthz
curl -X PUT http://127.0.0.1:8081/kv/client-1 -H 'Content-Type: application/json' -d '{"value":"Иван"}'
curl http://127.0.0.1:8081/kv/client-1
curl -X DELETE http://127.0.0.1:8081/kv/client-1
```

Остановка: `Control + C` в терминале сервера.

## Как вставить свой парсер

В [`cmd/sfddb/main.go`](cmd/sfddb/main.go) для каждого HTTP-метода задаётся
callback `GetParser`, `SetParser` или `DeleteParser`. Callback получает
`*http.Request` и возвращает `httpserver.Command` либо ошибку. Например,
`SetParser` можно заменить функцией, которая читает другой JSON или иной
формат запроса. Обработка команд подключается через `Executor` в том же
объекте `httpserver.Config`.

## Проверка проекта

```bash
go test ./...
```
