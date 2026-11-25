package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	if value := os.Getenv("LOG_LEVEL"); value != "" {
		level, err := zapcore.ParseLevel(value)
		if err != nil {
			return nil, err
		}
		config.Level = zap.NewAtomicLevelAt(level)
	}
	return config.Build()
}
