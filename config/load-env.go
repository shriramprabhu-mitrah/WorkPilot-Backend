package config

import (
	"os"
)

type Config struct {
	Database DatabaseConfig
	HTTP     HTTPConfig
	Redis    RedisConfig
	Logger   LoggerConfig
}

type DatabaseConfig struct {
	Host                 string
	Port                 string
	Username             string
	Password             string
	Name                 string
	SSLMode              string
	AutoMigrate          string
	PreferSimpleProtocol string
}

type HTTPConfig struct {
	Port string
}

type LoggerConfig struct {
	Type  string
	Level string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

func LoadEnv() *Config {

	// Load environment variables and populate the Config struct

	return &Config{
		Database: DatabaseConfig{
			Host:        GetEnv("DB_HOST", ""),
			Port:        GetEnv("DB_PORT", "5432"),
			Username:    GetEnv("DB_USERNAME", ""),
			Password:    GetEnv("DB_PASSWORD", ""),
			Name:        GetEnv("DB_NAME", ""),
			SSLMode:              GetEnv("DB_SSL_MODE", ""),
			AutoMigrate:          GetEnv("DB_AUTOMIGRATE", "false"),
			PreferSimpleProtocol: GetEnv("DB_PREFER_SIMPLE_PROTOCOL", "false"),
		},
		HTTP: HTTPConfig{
			Port: GetEnv("HTTP_PORT", "6369"),
		},
		Logger: LoggerConfig{
			Type:  GetEnv("LOGGER_TYPE", "development"),
			Level: GetEnv("LOGGER_LEVEL", "debug"),
		},
		Redis: RedisConfig{
			Host:     GetEnv("REDIS_HOST", "localhost"),
			Port:     GetEnv("REDIS_PORT", "6379"),
			Password: GetEnv("REDIS_PASSWORD", ""),
		},
	}
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
