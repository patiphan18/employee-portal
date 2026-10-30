package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// DefaultJWTSecret is the development fallback. Production refuses to start with it.
const DefaultJWTSecret = "dev-secret-change-me"

type Config struct {
	Env                  string
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	JWTTTLMinutes        int
	HistoryRetentionDays int
	MaxUploadBytes       int64
	UploadDir            string
	CORSOrigins          []string
	// TrustProxyHeaders makes the server read the client address from
	// X-Real-IP / X-Forwarded-For. Only enable it when a proxy you control
	// overwrites those headers, otherwise clients can forge them.
	TrustProxyHeaders bool
	// SeedDemoUsers controls the built-in demo accounts. It is forced off in
	// production so a deployment never carries accounts with published passwords.
	SeedDemoUsers bool
}

// IsProduction reports whether production safety rules apply.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func Load() (*Config, error) {
	c := &Config{
		Env:                  env("APP_ENV", "development"),
		Port:                 env("PORT", "8000"),
		DatabaseURL:          env("DATABASE_URL", "postgres://portal:portal@localhost:5432/portal?sslmode=disable"),
		JWTSecret:            env("JWT_SECRET", DefaultJWTSecret),
		JWTTTLMinutes:        envInt("JWT_TTL_MINUTES", 60),
		HistoryRetentionDays: envInt("HISTORY_RETENTION_DAYS", 7),
		MaxUploadBytes:       int64(envInt("MAX_UPLOAD_BYTES", 2*1024*1024)),
		UploadDir:            env("UPLOAD_DIR", "./data/uploads"),
		CORSOrigins:          splitList(env("CORS_ORIGINS", "http://localhost:5173,http://localhost:8080")),
		TrustProxyHeaders:    envBool("TRUST_PROXY_HEADERS", false),
		SeedDemoUsers:        envBool("SEED_DEMO_USERS", true),
	}
	// Demo accounts ship with passwords documented in the README, so production
	// never gets them regardless of what SEED_DEMO_USERS says.
	if c.IsProduction() {
		c.SeedDemoUsers = false
	}
	return c, c.Validate()
}

func (c *Config) Validate() error {
	if c.Env == "production" && c.JWTSecret == DefaultJWTSecret {
		return errors.New("JWT_SECRET must be set to a non-default value when APP_ENV=production")
	}
	if c.JWTTTLMinutes <= 0 {
		return errors.New("JWT_TTL_MINUTES must be positive")
	}
	if c.HistoryRetentionDays <= 0 {
		return errors.New("HISTORY_RETENTION_DAYS must be positive")
	}
	if c.MaxUploadBytes <= 0 {
		return errors.New("MAX_UPLOAD_BYTES must be positive")
	}
	return nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
