package configs

import "strconv"

type Config struct {
	Database DatabaseConfig
	HTTP     HTTPConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Name     string
}

type HTTPConfig struct {
	Port int
}

func LoadEnv() *Config {
	// Load environment variables and populate the Config struct
	portInt, err := strconv.Atoi(getEnv("DB_PORT"))
	if err != nil {
		panic("Invalid DB_PORT value")
	}

	httpPortInt, err := strconv.Atoi(getEnv("HTTP_PORT"))
	if err != nil {
		panic("Invalid HTTP_PORT value")
	}

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST"),
			Port:     portInt,
			Username: getEnv("DB_USERNAME"),
			Password: getEnv("DB_PASSWORD"),
			Name:     getEnv("DB_NAME"),
		},
		HTTP: HTTPConfig{
			Port: httpPortInt,
		},
	}
}

func getEnv(key string) string {
	return ""
}
