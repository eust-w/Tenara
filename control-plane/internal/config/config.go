package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	LogLevel    string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        getenvOr("PORT", "8080"),
		LogLevel:    getenvOr("LOG_LEVEL", "info"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
