package config

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

// requiredSecrets lists the env-var names for secrets that MUST be provided.
// The app will exit at startup if any are empty.
var requiredSecrets = []string{"JWT_SECRET", "ENCRYPT_KEY", "HMAC_SIGNING_KEY"}

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	EncryptKey     string
	HMACKey        string
	LogLevel       string
	DataDir        string
	MigrationsPath string
	TenantID       string
}

func Load() *Config {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://medops:medops@localhost:5432/medops?sslmode=disable"),
		JWTSecret:      loadSecret("JWT_SECRET", "MedOps/JWTSecret"),
		EncryptKey:     loadSecret("ENCRYPT_KEY", "MedOps/EncryptKey"),
		HMACKey:        loadSecret("HMAC_SIGNING_KEY", "MedOps/HMACKey"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		DataDir:        getEnv("DATA_DIR", "/data/medops"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "/app/migrations"),
		TenantID:       getEnv("TENANT_ID", "default"),
	}
	enforceRequiredSecrets(cfg)
	return cfg
}

// loadSecret resolves a secret using a two-tier priority:
//  1. OS credential store (Windows Credential Manager on Windows, no-op elsewhere)
//  2. Environment variable
//
// Returns an empty string if neither source provides a value.
// The caller (enforceRequiredSecrets) will fatal if the result is empty.
func loadSecret(envKey, credTarget string) string {
	// Tier 1: OS credential vault (Windows-only; returns "" on other platforms)
	if val, err := loadFromCredentialStore(credTarget); err == nil && val != "" {
		return val
	}
	// Tier 2: environment variable
	return os.Getenv(envKey)
}

// enforceRequiredSecrets fatals if any required secret is empty, preventing
// the application from starting without proper secret provisioning.
func enforceRequiredSecrets(cfg *Config) {
	missing := false
	secrets := map[string]string{
		"JWT_SECRET":       cfg.JWTSecret,
		"ENCRYPT_KEY":      cfg.EncryptKey,
		"HMAC_SIGNING_KEY": cfg.HMACKey,
	}
	for _, key := range requiredSecrets {
		if secrets[key] == "" {
			logrus.WithField("env_var", key).Fatal(
				"SECURITY: required secret is not set — set this environment variable before starting the application",
			)
			missing = true
		}
	}
	if missing {
		os.Exit(1)
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
