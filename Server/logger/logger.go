package logger

import (
	"strings"

	"github.com/G6kco/CyberSpace/internal/types"
	"go.uber.org/zap"
)

func LoadLogger(env string) {
	var err error
	var Log *zap.Logger

	normalized := strings.ToLower(strings.TrimSpace(env))

	switch normalized {
	case "development":
		Log, err = zap.NewDevelopment()
	case "production":
		Log, err = zap.NewProduction()
	default:
		Log, err = zap.NewDevelopment()
		normalized = "development"
	}
	
	if err != nil{
		panic("Failed to load logger : " + err.Error())
	}
	
	types.LOG = Log
	Log.Info("Logger Initialized", zap.String("Environment", normalized))
}
