package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	JWTSecret    string
	ServiceToken string
	CookieSecure bool
}

func Load() (Config, error) {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg := Config{
		Addr:         valueOrDefault("ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		ServiceToken: os.Getenv("SERVICE_TOKEN"),
	}

	cfg.CookieSecure = true
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("COOKIE_SECURE must be true or false")
		}
		cfg.CookieSecure = parsed
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
