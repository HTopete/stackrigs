package config

import (
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabasePath   string
	AllowedOrigins []string
	Env            string

	// WebAuthn
	WebAuthnDisplayName string
	WebAuthnRPID        string
	WebAuthnRPOrigins   []string

	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GitHubCallbackURL  string

	// Session
	SessionSecret string
	SessionMaxAge int // seconds

	// Base URL for badge generation and redirects
	BaseURL string
}

func Load() *Config {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabasePath:   getEnv("DATABASE_PATH", "./stackrigs.db"),
		AllowedOrigins: parseOrigins(getEnv("ALLOWED_ORIGINS", "https://stackrigs.com,https://api.stackrigs.com,http://localhost:3000,http://localhost:5173")),
		Env:            getEnv("ENV", "dev"),

		WebAuthnDisplayName: getEnv("WEBAUTHN_DISPLAY_NAME", "StackRigs"),
		WebAuthnRPID:        getEnv("WEBAUTHN_RP_ID", "localhost"),
		WebAuthnRPOrigins:   parseOrigins(getEnv("WEBAUTHN_RP_ORIGINS", "http://localhost:3000,http://localhost:5173")),

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubCallbackURL:  getEnv("GITHUB_CALLBACK_URL", "http://localhost:8080/api/auth/github/callback"),

		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production-32chars!"),
		SessionMaxAge: 86400 * 7, // 7 days

		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),
	}
	return cfg
}

func (c *Config) IsProd() bool {
	return c.Env == "prod" || c.Env == "production"
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
