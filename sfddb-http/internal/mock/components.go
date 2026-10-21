// Package mock supplies replaceable stand-ins for the non-HTTP HLD components.
// Nothing here writes to disk or sends data to a replica.
package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"sfddb-http/internal/httpserver"
)

// Trace makes the route through the mock components visible to tests and demos.
type Trace struct {
	mu    sync.Mutex
	steps []string
}

func (t *Trace) add(step string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, step)
}

func (t *Trace) Steps() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.steps...)
}

// Translator stands in for the diagram's Language translator. Its Parse
// method is the callback injected into each HTTP handler.
type Translator struct{ Trace *Trace }

func (t *Translator) Parse(r *http.Request) (httpserver.Command, error) {
	t.Trace.add("translator")
	command := httpserver.Command{Key: r.PathValue("key")}
	if command.Key == "" {
		return command, errors.New("key is required")
	}
	switch r.Method {
	case http.MethodGet:
		command.Operation = "GET"
	case http.MethodPut:
		command.Operation = "SET"
		var body struct {
			Value string `json:"value"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			return command, fmt.Errorf("parse value: %w", err)
		}
		command.Value = body.Value
	case http.MethodDelete:
		command.Operation = "DELETE"
	default:
		return command, errors.New("unsupported method")
	}
	return command, nil
}

// Engine stands in for Storage Engine. Values exist only while this process runs.
type Engine struct {
	mu          sync.Mutex
	data        map[string]string
	Trace       *Trace
	Transaction *Transaction
}

func NewEngine(trace *Trace, tx *Transaction) *Engine {
	return &Engine{data: make(map[string]string), Trace: trace, Transaction: tx}
}

func (e *Engine) Execute(ctx context.Context, command httpserver.Command) (httpserver.Result, error) {
	e.Trace.add("storage engine")
	e.mu.Lock()
	defer e.mu.Unlock()
	switch command.Operation {
	case "GET":
		value, found := e.data[command.Key]
		return httpserver.Result{Value: value, Found: found}, nil
	case "SET":
		if err := e.Transaction.Apply(ctx, command); err != nil {
			return httpserver.Result{}, err
		}
		e.data[command.Key] = command.Value
		return httpserver.Result{Value: command.Value, Found: true}, nil
	case "DELETE":
		if _, found := e.data[command.Key]; !found {
			return httpserver.Result{Found: false}, nil
		}
		if err := e.Transaction.Apply(ctx, command); err != nil {
			return httpserver.Result{}, err
		}
		delete(e.data, command.Key)
		return httpserver.Result{Found: true}, nil
	default:
		return httpserver.Result{}, fmt.Errorf("unknown operation %q", command.Operation)
	}
}

// Transaction is a placeholder for the unspecified X / ACID component.
type Transaction struct {
	Trace *Trace
	WAL   *WAL
}

func (t *Transaction) Apply(ctx context.Context, command httpserver.Command) error {
	t.Trace.add("transaction (ACID mock)")
	return t.WAL.Append(ctx, command)
}

// WAL is a mock: it forwards the command but keeps no durable journal.
type WAL struct {
	Trace      *Trace
	Disk       *Disk
	Replicator *Replicator
}

func (w *WAL) Append(ctx context.Context, command httpserver.Command) error {
	w.Trace.add("WAL mock")
	if err := w.Disk.Write(ctx, command); err != nil {
		return err
	}
	return w.Replicator.Send(ctx, command)
}

type Disk struct{ Trace *Trace }

func (d *Disk) Write(ctx context.Context, _ httpserver.Command) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.Trace.add("disk mock")
	return nil
}

// Replicator stands in for Y → network → SFDDB replica.
type Replicator struct{ Trace *Trace }

func (r *Replicator) Send(ctx context.Context, _ httpserver.Command) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.Trace.add("Y -> network -> replica mock")
	return nil
}

// ServiceClient and AdminCLI represent the two clients in the diagram.
// They build requests for the same HTTP boundary; no separate CLI protocol exists.
type ServiceClient struct{ BaseURL string }
type AdminCLI struct{ BaseURL string }

func (c ServiceClient) Request(ctx context.Context, method, key, value string) (*http.Request, error) {
	return buildRequest(ctx, c.BaseURL, method, key, value)
}

func (c AdminCLI) Request(ctx context.Context, method, key, value string) (*http.Request, error) {
	return buildRequest(ctx, c.BaseURL, method, key, value)
}

func buildRequest(ctx context.Context, baseURL, method, key, value string) (*http.Request, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("key is required")
	}
	var body io.Reader
	if method == http.MethodPut {
		payload, err := json.Marshal(map[string]string{"value": value})
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(string(payload))
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+"/kv/"+url.PathEscape(key), body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}
