package config

import (
	"errors"
	"os"

	types "github.com/G6kco/CyberSpace/internal/types"
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string `env:"ENV,required"`
	ServerPort     uint   `env:"PORT"`
	DatabaseURL    string `env:"DATABASE_URL,required"`
	FrontendURL    string `env:"FRONTEND_URL"`
	SessionSecret  string `env:"SESSION_SECRET"`
	GoogleClientID string `env:"GOOGLE_CLIENT_ID"`
	GoogleSecret   string `env:"GOOGLE_SECRET"`
	GoogleCallback string `env:"GOOGLE_CALL_BACK"`
	DockerHost     string `env:"DOCKER_HOST"`
	CORSOrigins    string `env:"CORS_ORIGIN_ALLOWED"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		return nil, types.ParseError
	}

	return &cfg, nil
}
