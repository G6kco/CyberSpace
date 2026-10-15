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

	server := newHTTPServer("127.0.0.1", 8080, handler)

	if server.Addr != "127.0.0.1:8080" {
		t.Fatalf("Addr = %q, want %q", server.Addr, "127.0.0.1:8080")
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

func TestRunRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("DATABASE_URL", "")

	err := run()

	if err == nil {
		t.Fatal("run() error = nil, want a DATABASE_URL validation error")
	}
	if err.Error() != "load configuration: DATABASE_URL is required" {
		t.Fatalf("run() error = %q, want %q", err, "load configuration: DATABASE_URL is required")
	}
}

func TestRunReturnsDatabaseConnectionError(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("DATABASE_URL", "user:password@tcp(localhost:3306")

	err := run()

	if err == nil {
		t.Fatal("run() error = nil, want a database connection error")
	}
	if !strings.Contains(err.Error(), "connect to database") {
		t.Fatalf("run() error = %q, want it to contain %q", err, "connect to database")
	}
}

// setValidEnvironment installs a configuration that passes validation, so a
// test can break one variable and reach the startup stage it means to exercise.
// The values are placeholders in reserved test domains, not credentials.
func setValidEnvironment(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"APP_ENV":               "development",
		"SERVER_HOST":           "0.0.0.0",
		"SERVER_PORT":           "8080",
		"DATABASE_URL":          "user:password@tcp(localhost:3306)/cyberspace",
		"CORS_ORIGIN_ALLOWED":   "",
		"FRONTEND_URL":          "https://app.example.test",
		"SESSION_SECRET":        "test-session-secret",
		"GOOGLE_CLIENT_ID":      "test-client-id.apps.googleusercontent.com",
		"GOOGLE_SECRET":         "test-client-secret",
		"GOOGLE_CALL_BACK":      "https://api.example.test/api/v1/auth/google/callback",
		"GOOGLE_ALLOWED_DOMAIN": "example.test",
		"DOCKER_HOST":           "",
		"ANSWER_HMAC_KEY":       strings.Repeat("k", 32),
		"PCAP_DIR":              "",
	}

	for key, value := range values {
		t.Setenv(key, value)
	}
}
