package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const productionWorkerOrigin = "https://inmobiliariafront.joaquinr-val.workers.dev"

type Config struct {
	Addr           string
	DatabaseURL    string
	JWTSecret      string
	ServiceToken   string
	CookieSecure   bool
	AllowedOrigins []string
}

func Load() (Config, error) {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	allowedOrigins := splitList(valueOrDefault("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"))
	if !containsOrigin(allowedOrigins, productionWorkerOrigin) {
		allowedOrigins = append(allowedOrigins, productionWorkerOrigin)
	}

	cfg := Config{
		Addr:           resolveAddr(),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		ServiceToken:   os.Getenv("SERVICE_TOKEN"),
		AllowedOrigins: allowedOrigins,
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

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func containsOrigin(origins []string, target string) bool {
	for _, origin := range origins {
		if origin == target {
			return true
		}
	}
	return false
}

func resolveAddr() string {
	if value := os.Getenv("ADDR"); value != "" {
		return value
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}
