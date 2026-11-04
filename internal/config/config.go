package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	MediaBaseURL   string
	R2AccountID    string
	R2AccessKeyID  string
	R2SecretKey    string
	R2Bucket       string
	R2PresignTTL   time.Duration
	R2MaxUpload    int64
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
		MediaBaseURL:   strings.TrimRight(valueOrDefault("MEDIA_BASE_URL", productionWorkerOrigin+"/media"), "/"),
		R2AccountID:    os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:  os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretKey:    os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:       os.Getenv("R2_BUCKET"),
		R2PresignTTL:   time.Duration(envInt64("R2_PRESIGN_TTL", 300)) * time.Second,
		R2MaxUpload:    envInt64("R2_MAX_UPLOAD_BYTES", 8<<20),
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

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
