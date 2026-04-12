package config

import (
	"os"
	"time"
)

// Config holds all application settings loaded from environment variables.
type Config struct {
	Port         string
	Host         string
	DatabaseURL  string
	SMTPHost     string
	SMTPPort     string
	SMTPFrom     string
	SMTPUser     string
	SMTPPass     string
	BrevoAPIKey  string
	APIKey       string
	GitHubToken  string
	ScanInterval time.Duration
	RedisURL     string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	interval, err := time.ParseDuration(getEnv("SCAN_INTERVAL", "10m"))
	if err != nil {
		interval = 10 * time.Minute
	}
	return &Config{
		Port:         getEnv("PORT", "8080"),
		Host:         getEnv("HOST", "localhost:8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/genesis?sslmode=disable"),
		SMTPHost:     getEnv("SMTP_HOST", "localhost"),
		SMTPPort:     getEnv("SMTP_PORT", "1025"),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@genesis.app"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPass:     getEnv("SMTP_PASS", ""),
		BrevoAPIKey:  getEnv("BREVO_API_KEY", ""),
		APIKey:       getEnv("API_KEY", ""),
		GitHubToken:  getEnv("GITHUB_TOKEN", ""),
		ScanInterval: interval,
		RedisURL:     getEnv("REDIS_URL", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
