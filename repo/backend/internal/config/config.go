package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	EncryptKey  string
	LogLevel    string
	DataDir     string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://medops:medops@localhost:5432/medops?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "medops-local-secret-change-in-production"),
		EncryptKey:  getEnv("ENCRYPT_KEY", "0123456789abcdef0123456789abcdef"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DataDir:     getEnv("DATA_DIR", "/data/medops"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
