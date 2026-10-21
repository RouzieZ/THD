package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"sfddb-http/internal/httpserver"
	"sfddb-http/internal/mock"
)

func TestCallbacksAndMockChain(t *testing.T) {
	trace := &mock.Trace{}
	translator := &mock.Translator{Trace: trace}
	disk := &mock.Disk{Trace: trace}
	replica := &mock.Replicator{Trace: trace}
	w := &mock.WAL{Trace: trace, Disk: disk, Replicator: replica}
	engine := mock.NewEngine(trace, &mock.Transaction{Trace: trace, WAL: w})

	called := false
	setParser := func(r *http.Request) (httpserver.Command, error) {
		called = true
		return translator.Parse(r)
	}
	server, err := httpserver.New(httpserver.Config{
		GetParser:    translator.Parse,
		SetParser:    setParser,
		DeleteParser: translator.Parse,
		Executor:     engine,
	})
	if err != nil {
		t.Fatal(err)
	}

	service := mock.ServiceClient{BaseURL: "http://example.test"}
	request, err := service.Request(context.Background(), http.MethodPut, "client-1", "Иван")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !called {
		t.Fatalf("PUT status=%d, parser called=%v", recorder.Code, called)
	}
	want := []string{"translator", "storage engine", "transaction (ACID mock)", "WAL mock", "disk mock", "Y -> network -> replica mock"}
	if got := trace.Steps(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mock chain = %v, want %v", got, want)
	}

	request = httptest.NewRequest(http.MethodGet, "/kv/client-1", nil)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"value":"Иван"`) {
		t.Fatalf("GET status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	admin := mock.AdminCLI{BaseURL: "http://example.test"}
	request, err = admin.Request(context.Background(), http.MethodDelete, "client-1", "")
	if err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("DELETE status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
