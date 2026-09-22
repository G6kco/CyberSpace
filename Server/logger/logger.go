package logger

import (
	"strings"

	"go.uber.org/zap"
)

// New constructs a logger for the requested application environment.
func New(env string) (*zap.Logger, error) {
	normalized := strings.ToLower(strings.TrimSpace(env))

	var (
		log *zap.Logger
		err error
	)

	switch normalized {
	case "development":
		log, err = zap.NewDevelopment()
	case "production":
		log, err = zap.NewProduction()
	default:
		log, err = zap.NewDevelopment()
		normalized = "development"
	}

	if err != nil {
		return nil, err
	}

	log.Info("Logger initialized", zap.String("environment", normalized))
	return log, nil
}
