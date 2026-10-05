package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	setBaseEnvironment(t)
	t.Setenv("APP_ENV", "")
	t.Setenv("SERVER_HOST", "")
	t.Setenv("SERVER_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "0.0.0.0")
	}
	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want %d", cfg.ServerPort, 8080)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	setBaseEnvironment(t)
	t.Setenv("APP_ENV", " PRODUCTION ")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv(
		"CORS_ORIGIN_ALLOWED",
		"https://admin.example.com, https://student.example.com,https://admin.example.com",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "production")
	}
	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "127.0.0.1")
	}
	if cfg.ServerPort != 9090 {
		t.Errorf("ServerPort = %d, want %d", cfg.ServerPort, 9090)
	}

	wantOrigins := []string{
		"https://admin.example.com",
		"https://student.example.com",
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins = %#v, want %#v", cfg.AllowedOrigins, wantOrigins)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name: "missing database URL",
			key:  "DATABASE_URL",
			want: "DATABASE_URL is required",
		},
		{
			name:  "unknown application environment",
			key:   "APP_ENV",
			value: "staging",
			want:  `APP_ENV must be development or production, got "staging"`,
		},
		{
			name:  "non-numeric server port",
			key:   "SERVER_PORT",
			value: "http",
			want:  "SERVER_PORT must be a number between 1 and 65535",
		},
		{
			name:  "server port above range",
			key:   "SERVER_PORT",
			value: "65536",
			want:  "SERVER_PORT must be a number between 1 and 65535",
		},
		{
			name:  "invalid allowed origin",
			key:   "CORS_ORIGIN_ALLOWED",
			value: "admin.example.com/path",
			want:  `CORS_ORIGIN_ALLOWED contains invalid origin "admin.example.com/path"`,
		},
		{
			name: "missing Google client ID",
			key:  "GOOGLE_CLIENT_ID",
			want: "Google_Client_ID is required",
		},
		{
			name: "missing Google secret",
			key:  "GOOGLE_SECRET",
			want: "Google_secret is required",
		},
		{
			name: "missing Google allowed domain",
			key:  "GOOGLE_ALLOWED_DOMAIN",
			want: "Google_Allowed_Domains is required",
		},
		{
			name: "missing frontend URL",
			key:  "FRONTEND_URL",
			want: "Frontend_URL is required",
		},
		{
			name:  "callback is not the API callback path",
			key:   "GOOGLE_CALL_BACK",
			value: "https://api.example.test/oauth2/callback",
			want:  "Google_Call_back should be API callback URL",
		},
		{
			name:  "frontend URL is not absolute",
			key:   "FRONTEND_URL",
			value: "/app",
			want:  "The Frontend URL must be an absolute URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setBaseEnvironment(t)
			t.Setenv(test.key, test.value)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want a validation error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

// setBaseEnvironment installs a complete, valid configuration so that each test
// can invalidate exactly one variable. Every variable Load reads is set here,
// including the ones left empty, so tests never inherit the developer's shell.
//
// These are placeholders in reserved test domains, not credentials. The tests
// exercise validation rules, never a real Google account.
func setBaseEnvironment(t *testing.T) {
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
	}

	for key, value := range values {
		t.Setenv(key, value)
	}
}
