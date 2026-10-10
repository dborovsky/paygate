package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	address := getEnv("HTTP_ADDR", ":8080")

	database := getEnv("DATABASE_URL", "postgres://paygate:paygate@localhost:5432/paygate?sslmode=disable")

	t := getEnv("SHUTDOWN_TIMEOUT", "10s")

	duration, err := time.ParseDuration(t)
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	return Config{
		HTTPAddr:        address,
		DatabaseURL:     database,
		ShutdownTimeout: duration,
	}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return fallback
}
