package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"ORCAN_API_ADDR", "PAYMENT_API_ADDR", "CLERK_SECRET_KEY", "INTERNAL_API_TOKEN"} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.Port != "8082" || cfg.DBName != "pingu" || cfg.OrcanAddr != "localhost:8080" || cfg.PaymentAddr != "localhost:8081" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	// 秘密情報は、未設定のまま既定値で動いてはいけない。
	if cfg.ClerkSecretKey != "" || cfg.InternalAPIToken != "" || cfg.DatabaseURL != "" {
		t.Errorf("secrets must default to empty: %+v", cfg)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PORT", "9002")
	t.Setenv("ORCAN_API_ADDR", "orcan-api:8080")
	t.Setenv("PAYMENT_API_ADDR", "payment-api:8081")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_x")
	t.Setenv("INTERNAL_API_TOKEN", "shared-secret")

	cfg := Load()

	if cfg.Port != "9002" || cfg.OrcanAddr != "orcan-api:8080" || cfg.PaymentAddr != "payment-api:8081" ||
		cfg.ClerkSecretKey != "sk_test_x" || cfg.InternalAPIToken != "shared-secret" {
		t.Errorf("environment was not applied: %+v", cfg)
	}
}

func TestEmptyEnvironmentValueFallsBack(t *testing.T) {
	t.Setenv("ORCAN_API_ADDR", "")
	if got := Load().OrcanAddr; got != "localhost:8080" {
		t.Errorf("OrcanAddr = %q, want the default", got)
	}
}
