package logger

import (
	"fmt"

	"github.com/ms-kanban-server/config"
	"go.uber.org/zap"
)

func InitLogger(config *config.Config) (*zap.Logger, error) {

	var Log *zap.Logger
	var err error

	//Set flag for the logger type #default development
	if config.Logger.Type == "production" {
		Log, err = zap.NewProduction(zap.AddStacktrace(zap.DPanicLevel))
	} else {
		Log, err = zap.NewDevelopment(zap.AddStacktrace(zap.DPanicLevel))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return Log, nil
}
