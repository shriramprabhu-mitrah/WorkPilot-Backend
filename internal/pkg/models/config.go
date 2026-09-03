package models

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Config struct {
	Database *gorm.DB
	Router   *gin.Engine
	Redis    *redis.Client
	Logger   *zap.Logger
	Context  context.Context
}

type AttachmentConfig struct {
	MaxFileSizeMB int64
	MaxFiles      int
}
