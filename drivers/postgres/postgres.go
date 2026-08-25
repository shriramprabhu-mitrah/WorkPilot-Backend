package postgres

import (
	"fmt"
	"strings"
	"time"

	configs "github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/drivers/migration"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDB(config *configs.Config) (*gorm.DB, error) {
	connectionString := buildConnectionString(config)

	preferSimple := strings.EqualFold(config.Database.PreferSimpleProtocol, "true")

	dbConn, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  connectionString,
		PreferSimpleProtocol: preferSimple,
	}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	sqlDB, err := dbConn.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to access database connection pool: %w", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if config.Database.AutoMigrate == "true" {
		err = migration.AutoMigrate(dbConn)
		if err != nil {
			return nil, fmt.Errorf("failed to auto-migrate the database: %w", err)
		}
	}

	return dbConn, nil
}

func buildConnectionString(config *configs.Config) string {
	sslMode := strings.TrimSpace(config.Database.SSLMode)
	if sslMode == "" {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Database.Host,
		config.Database.Port,
		config.Database.Username,
		config.Database.Password,
		config.Database.Name,
		sslMode,
	)
}
