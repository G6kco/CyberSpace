package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type fakeDatabase struct {
	pingError error
}

func (database fakeDatabase) PingContext(context.Context) error {
	return database.pingError
}

func TestHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		pingError  error
		wantStatus int
		wantBody   string
		wantLog    string
	}{
		{
			name:       "live",
			path:       "/health/live",
			wantStatus: http.StatusOK,
			wantBody:   `"status":"alive"`,
			wantLog:    "Liveness check succeeded",
		},
		{
			name:       "ready",
			path:       "/health/ready",
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ready"`,
			wantLog:    "Readiness check succeeded",
		},
		{
			name:       "database unavailable",
			path:       "/health/ready",
			pingError:  errors.New("database offline"),
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"status":"not_ready"`,
			wantLog:    "Readiness check failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			application, err := app.New(
				&config.Config{},
				fakeDatabase{pingError: test.pingError},
				zap.New(core),
			)
			if err != nil {
				t.Fatalf("create application: %v", err)
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			NewRouter(application).ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), test.wantBody) {
				t.Errorf("body = %q, want it to contain %q", recorder.Body.String(), test.wantBody)
			}
			if logs.FilterMessage(test.wantLog).Len() != 1 {
				t.Errorf("log %q was not emitted exactly once", test.wantLog)
			}
		})
	}
}
