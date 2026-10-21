// Package httpserver contains only the HTTP boundary of SFDDB.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Command is the transport-independent request passed to the next component.
type Command struct {
	Operation string
	Key       string
	Value     string
}

// Result is returned by the downstream component. Found is used by GET/DELETE.
type Result struct {
	Value string
	Found bool
}

// Parser is called by an HTTP handler. Replace it to use another request format.
type Parser func(*http.Request) (Command, error)

// Executor represents the language translator / storage pipeline behind HTTP.
type Executor interface {
	Execute(context.Context, Command) (Result, error)
}

// Config is supplied as a Go object when constructing Server.
type Config struct {
	Address           string
	ReadHeaderTimeout time.Duration
	GetParser         Parser
	SetParser         Parser
	DeleteParser      Parser
	Executor          Executor
}

// Server owns the standard-library HTTP server and its handlers.
type Server struct {
	config Config
	http   *http.Server
	logger *log.Logger
}

// New validates the injected components and creates a server.
func New(config Config) (*Server, error) {
	if config.GetParser == nil || config.SetParser == nil || config.DeleteParser == nil {
		return nil, errors.New("get, set and delete parsers are required")
	}
	if config.Executor == nil {
		return nil, errors.New("executor is required")
	}
	if config.Address == "" {
		config.Address = "127.0.0.1:8081"
	}
	if config.ReadHeaderTimeout <= 0 {
		config.ReadHeaderTimeout = 5 * time.Second
	}

	s := &Server{config: config, logger: log.New(os.Stdout, "sfddb-http: ", log.LstdFlags)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /kv/{key}", s.get)
	mux.HandleFunc("PUT /kv/{key}", s.set)
	mux.HandleFunc("DELETE /kv/{key}", s.delete)
	s.http = &http.Server{Addr: config.Address, Handler: mux, ReadHeaderTimeout: config.ReadHeaderTimeout}
	return s, nil
}

// Handler exposes the routes for composition or httptest without binding a port.
func (s *Server) Handler() http.Handler { return s.http.Handler }

// Run serves HTTP until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			s.logger.Printf("shutdown error: %v", err)
		}
	}()
	defer close(done)
	s.logger.Printf("listening on %s", s.config.Address)
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "OK",
		"name":   "SFDDB",
	})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, s.config.GetParser)
}

func (s *Server) set(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, s.config.SetParser)
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r, s.config.DeleteParser)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request, parse Parser) {
	command, err := parse(r)
	if err != nil {
		s.logger.Printf("invalid %s %s: %v", r.Method, r.URL.Path, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.config.Executor.Execute(r.Context(), command)
	if err != nil {
		s.logger.Printf("execute %s: %v", command.Operation, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	s.logger.Printf("%s key=%q", command.Operation, command.Key)
	if (command.Operation == "GET" || command.Operation == "DELETE") && !result.Found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "kEy n0t found"})
		return
	}
	if command.Operation == "DELETE" {
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": command.Key, "value": result.Value})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Encoding the small response objects above should never fail.
		fmt.Fprintln(os.Stdout, "sfddb-http: response encoding error:", err)
	}
}
