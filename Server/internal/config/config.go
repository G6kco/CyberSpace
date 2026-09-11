package config

import (
	"fmt"

	types "github.com/G6kco/CyberSpace.git/internal/types"
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string `env:"ENV,required"`
	ServerPort     uint `env:"PORT"`
	DatabaseURL    string `env:"DATABASE_URL,required"`
	FrontendURL    string	`env:"FRONTEND_URL,required"`
	SessionSecret  string	`env:"SESSION_SECRET,required"`
	GoogleClientID string	`env:"GOOGLE_CLIENT_ID,required"`
	GoogleSecret   string	`env:"GOOGLE_SECRET, required"`
	GoogleCallback string	`env:"GOOGLE_CALL_BACK, required"`
	DockerHost     string	`env:"DOCKER_HOST"`
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil{
		fmt.Println("No .env file found, using OS environment variables")
		return nil, err
	}else {
		fmt.Println("Successfully loaded the env file, using defined environment variables")
	}
	
	cfg := Config{}
	
	if err := env.Parse(&cfg); err != nil{
		return nil, types.ParseError
	}
	
	return &cfg, nil
}