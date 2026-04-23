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

	// Base URL of the Go API (used internally)
	BaseURL string

	// Frontend URL for post-auth redirects (e.g. https://stackrigs.com)
	FrontendURL string

	// CookieDomain sets the Domain attribute on session cookies.
	// Use ".stackrigs.com" in production so cookies are shared
	// between stackrigs.com (frontend) and api.stackrigs.com (API).
	CookieDomain string
}

func Load() *Config {
	env := getEnv("ENV", "dev")
	isProd := env == "prod" || env == "production"
	allowedOrigins := parseOrigins(getEnv("ALLOWED_ORIGINS", "https://stackrigs.com,https://api.stackrigs.com,http://localhost:3000,http://localhost:5173"))
	if isProd {
		allowedOrigins = filterProdOrigins(allowedOrigins)
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabasePath:   getEnv("DATABASE_PATH", "./stackrigs.db"),
		AllowedOrigins: allowedOrigins,
		Env:            env,

		WebAuthnDisplayName: getEnv("WEBAUTHN_DISPLAY_NAME", "StackRigs"),
		WebAuthnRPID:        getEnv("WEBAUTHN_RP_ID", "localhost"),
		WebAuthnRPOrigins:   parseOrigins(getEnv("WEBAUTHN_RP_ORIGINS", "http://localhost:3000,http://localhost:5173")),

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubCallbackURL:  getEnv("GITHUB_CALLBACK_URL", "http://localhost:8080/api/auth/github/callback"),

		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production-32chars!"),
		SessionMaxAge: 86400 * 7, // 7 days

		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
		FrontendURL: getEnv("FRONTEND_URL", getEnv("BASE_URL", "http://localhost:8080")),

		CookieDomain: getEnv("COOKIE_DOMAIN", ""),
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

// filterProdOrigins removes any localhost/127.0.0.1 origins so they can never
// leak into production even if ALLOWED_ORIGINS is not explicitly set.
func filterProdOrigins(origins []string) []string {
	filtered := make([]string, 0, len(origins))
	for _, o := range origins {
		if strings.Contains(o, "localhost") || strings.Contains(o, "127.0.0.1") || strings.Contains(o, "::1") {
			continue
		}
		filtered = append(filtered, o)
	}
	return filtered
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
