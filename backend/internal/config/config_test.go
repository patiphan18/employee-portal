package config_test

import (
	"testing"

	"github.com/digithun/employee-portal/backend/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8000" {
		t.Errorf("Port = %q, want 8000", cfg.Port)
	}
	if cfg.HistoryRetentionDays != 7 {
		t.Errorf("HistoryRetentionDays = %d, want 7", cfg.HistoryRetentionDays)
	}
	if cfg.MaxUploadBytes != 2*1024*1024 {
		t.Errorf("MaxUploadBytes = %d, want 2 MB", cfg.MaxUploadBytes)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("CORSOrigins = %v, want 2 entries", cfg.CORSOrigins)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("HISTORY_RETENTION_DAYS", "30")
	t.Setenv("CORS_ORIGINS", "https://a.example.com , https://b.example.com ,")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("Port = %q", cfg.Port)
	}
	if cfg.HistoryRetentionDays != 30 {
		t.Errorf("HistoryRetentionDays = %d", cfg.HistoryRetentionDays)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://a.example.com" {
		t.Errorf("CORSOrigins = %v, want trimmed entries with the blank dropped", cfg.CORSOrigins)
	}
}

func TestNonNumericEnvFallsBackToDefault(t *testing.T) {
	t.Setenv("JWT_TTL_MINUTES", "sixty")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTTTLMinutes != 60 {
		t.Errorf("JWTTTLMinutes = %d, want the 60-minute default", cfg.JWTTTLMinutes)
	}
}

func TestProductionRefusesTheDefaultSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if _, err := config.Load(); err == nil {
		t.Fatal("production with the default JWT secret must fail to start")
	}

	t.Setenv("JWT_SECRET", "a-real-secret-from-the-vault")
	if _, err := config.Load(); err != nil {
		t.Errorf("production with a real secret should start, got %v", err)
	}
}

func TestValidateRejectsNonPositiveNumbers(t *testing.T) {
	cases := []config.Config{
		{JWTTTLMinutes: 0, HistoryRetentionDays: 7, MaxUploadBytes: 1},
		{JWTTTLMinutes: 60, HistoryRetentionDays: 0, MaxUploadBytes: 1},
		{JWTTTLMinutes: 60, HistoryRetentionDays: 7, MaxUploadBytes: 0},
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d should be invalid", i)
		}
	}
}

// Regression for the review finding "demo admin account is seeded in production".
// The demo accounts' passwords are published in the README, so production must
// never get them, whatever the environment says.
func TestProductionNeverSeedsDemoUsers(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "a-real-secret-from-the-vault")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SeedDemoUsers {
		t.Error("production must not seed demo users")
	}

	// Even an explicit opt-in is overridden.
	t.Setenv("SEED_DEMO_USERS", "true")
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SeedDemoUsers {
		t.Error("SEED_DEMO_USERS=true must not re-enable seeding in production")
	}
}

func TestDemoUsersSeededInDevelopmentByDefault(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.SeedDemoUsers {
		t.Error("development should seed demo users so the quick start works")
	}
	if cfg.TrustProxyHeaders {
		t.Error("proxy headers must not be trusted unless explicitly enabled")
	}
}

func TestSeedDemoUsersCanBeDisabledOutsideProduction(t *testing.T) {
	t.Setenv("SEED_DEMO_USERS", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SeedDemoUsers {
		t.Error("SEED_DEMO_USERS=false should disable seeding")
	}
}

func TestTrustProxyHeadersParsing(t *testing.T) {
	for _, v := range []string{"true", "1", "TRUE"} {
		t.Setenv("TRUST_PROXY_HEADERS", v)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load(%q): %v", v, err)
		}
		if !cfg.TrustProxyHeaders {
			t.Errorf("TRUST_PROXY_HEADERS=%q should enable the flag", v)
		}
	}
	// An unparseable value keeps the safe default rather than guessing.
	t.Setenv("TRUST_PROXY_HEADERS", "yes-please")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TrustProxyHeaders {
		t.Error("an invalid value must fall back to not trusting headers")
	}
}
