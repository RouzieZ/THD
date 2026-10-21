package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sfddb-http/internal/httpserver"
	"sfddb-http/internal/mock"
)

func main() {
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8081"
	}

	trace := &mock.Trace{}
	translator := &mock.Translator{Trace: trace}
	disk := &mock.Disk{Trace: trace}
	replica := &mock.Replicator{Trace: trace}
	wal := &mock.WAL{Trace: trace, Disk: disk, Replicator: replica}
	transaction := &mock.Transaction{Trace: trace, WAL: wal}
	engine := mock.NewEngine(trace, transaction)

	server, err := httpserver.New(httpserver.Config{
		Address:      address,
		GetParser:    translator.Parse,
		SetParser:    translator.Parse,
		DeleteParser: translator.Parse,
		Executor:     engine,
	})
	if err != nil {
		log.New(os.Stdout, "sfddb-http: ", log.LstdFlags).Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx); err != nil {
		log.New(os.Stdout, "sfddb-http: ", log.LstdFlags).Fatal(err)
	}
}
