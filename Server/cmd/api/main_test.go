package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPServer(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})

	server := newHTTPServer(8080, handler)

	if server.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", server.Addr, ":8080")
	}
	if server.Handler == nil {
		t.Fatal("Handler is nil")
	}
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Errorf("handler status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, 5*time.Second)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %s, want %s", server.ReadTimeout, 15*time.Second)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %s, want %s", server.WriteTimeout, 30*time.Second)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %s, want %s", server.IdleTimeout, 60*time.Second)
	}
}

func TestRunRejectsEmptyAppEnvironment(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("DATABASE_URL", "")

	err := run()

	if err == nil {
		t.Fatal("run() error = nil, want an APP_ENV validation error")
	}
	if err.Error() != "APP_ENV cannot be empty" {
		t.Fatalf("run() error = %q, want %q", err, "APP_ENV cannot be empty")
	}
}

func TestRunReturnsDatabaseConnectionError(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "user:password@tcp(localhost:3306")

	err := run()

	if err == nil {
		t.Fatal("run() error = nil, want a database connection error")
	}
	if !strings.Contains(err.Error(), "connect to database") {
		t.Fatalf("run() error = %q, want it to contain %q", err, "connect to database")
	}
}
