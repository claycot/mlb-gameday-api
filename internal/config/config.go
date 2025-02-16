package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           int
	Hostname       string
	AllowedOrigins []string
}

// load the config, with defaults if the .env file doesn't exist or values are not provided
func Load(logger *log.Logger) (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		logger.Printf("Failed to load .env file: %e\r\n", err)
	}

	port, err := strconv.Atoi(getEnv("PORT_", "8080"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:           port,
		Hostname:       getEnv("HOSTNAME_", ""),
		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ","),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
