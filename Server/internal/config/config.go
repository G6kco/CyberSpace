// Package config loads, defaults, and validates application configuration.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAppEnv     = "development"
	defaultServerHost = "0.0.0.0"
	defaultServerPort = 8080
	// defaultCallBackURL = "http://localhost:8080/auth/google/callback"
)

type Config struct {
	AppEnv              string
	ServerHost          string
	ServerPort          uint
	DatabaseURL         string
	AllowedOrigins      []string
	FrontendURL         string
	SessionSecret       string
	GoogleClientID      string
	GoogleSecret        string
	GoogleCallback      string
	DockerHost          string
	GoogleAllowedDomain string
}

// Load reads the optional .env file and the process environment, applies safe
// defaults, and rejects invalid configuration before startup can continue.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env file: %w", err)
	}

	cfg, err := loadEnvironment()
	if err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadEnvironment performs only environment loading. Defaults and validation
// remain separate stages so their behavior is predictable and testable.
func loadEnvironment() (*Config, error) {
	var serverPort uint
	if rawPort := strings.TrimSpace(os.Getenv("SERVER_PORT")); rawPort != "" {
		parsedPort, err := strconv.ParseUint(rawPort, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("SERVER_PORT must be a number between 1 and 65535: %w", err)
		}
		serverPort = uint(parsedPort)
	}

	return &Config{
		AppEnv:         strings.TrimSpace(os.Getenv("APP_ENV")),
		ServerHost:     strings.TrimSpace(os.Getenv("SERVER_HOST")),
		ServerPort:     serverPort,
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AllowedOrigins: parseAllowedOrigins(os.Getenv("CORS_ORIGIN_ALLOWED")),
		FrontendURL:    strings.TrimSpace(os.Getenv("FRONTEND_URL")),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleSecret:   os.Getenv("GOOGLE_SECRET"),
		GoogleCallback: strings.TrimSpace(os.Getenv("GOOGLE_CALL_BACK")),
		DockerHost:     strings.TrimSpace(os.Getenv("DOCKER_HOST")),
		GoogleAllowedDomain: strings.ToLower(
			strings.TrimSpace(os.Getenv("GOOGLE_ALLOWED_DOMAIN")),
		),
	}, nil
}

func (cfg *Config) applyDefaults() {
	if cfg.AppEnv == "" {
		cfg.AppEnv = defaultAppEnv
	}
	if cfg.ServerHost == "" {
		cfg.ServerHost = defaultServerHost
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = defaultServerPort
	}

	cfg.AppEnv = strings.ToLower(cfg.AppEnv)
}

func (cfg *Config) validate() error {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}

	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		return fmt.Errorf(
			"APP_ENV must be development or production, got %q",
			cfg.AppEnv,
		)
	}

	if strings.TrimSpace(cfg.ServerHost) == "" {
		return errors.New("SERVER_HOST cannot be empty")
	}

	if cfg.ServerPort == 0 || cfg.ServerPort > 65535 {
		return errors.New("SERVER_PORT must be between 1 and 65535")
	}

	if cfg.GoogleClientID == "" {
		return errors.New("Google_Client_ID is required")
	}

	if cfg.GoogleSecret == "" {
		return errors.New("Google_secret is required")
	}

	if cfg.GoogleCallback == "" {
		return errors.New("Google_call_back is reuired")
	}

	if cfg.GoogleAllowedDomain == "" {
		return errors.New("Google_Allowed_Domains is required")
	}

	if cfg.FrontendURL == "" {
		return errors.New("Frontend_URL is required")
	}

	callback, err := url.Parse(cfg.GoogleCallback)
	if err != nil || callback.Host == "" || callback.Path != "/api/v1/auth/google/callback" {
		return errors.New("Google_Call_back should be API callback URL")
	}

	frontend, err := url.Parse(cfg.FrontendURL)
	if err != nil || frontend.Host == "" {
		return errors.New("The Frontend URL must be an absolute URL")
	}

	if cfg.AppEnv == "production" && (callback.Scheme != "https" || frontend.Scheme != "https") {
		return errors.New("prodcution URLs must be HTTPS")
	}

	for _, origin := range cfg.AllowedOrigins {
		parsedOrigin, err := url.Parse(origin)
		if err != nil ||
			(parsedOrigin.Scheme != "http" && parsedOrigin.Scheme != "https") ||
			parsedOrigin.Host == "" ||
			(parsedOrigin.Path != "" && parsedOrigin.Path != "/") ||
			parsedOrigin.RawQuery != "" ||
			parsedOrigin.Fragment != "" {
			return fmt.Errorf("CORS_ORIGIN_ALLOWED contains invalid origin %q", origin)
		}
	}

	return nil
}

func parseAllowedOrigins(rawOrigins string) []string {
	seen := make(map[string]struct{})
	origins := make([]string, 0)

	for _, rawOrigin := range strings.Split(rawOrigins, ",") {
		origin := strings.TrimSpace(rawOrigin)
		if origin == "" {
			continue
		}
		if _, exists := seen[origin]; exists {
			continue
		}

		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	return origins
}
