package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "INTERNAL_API_TOKEN"} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.Port != "8081" || cfg.DBHost != "localhost" || cfg.DBPort != "5432" || cfg.DBName != "payment" || cfg.DBSSLMode != "disable" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	// トークンが未設定のまま起動してはいけないので、デフォルトは空(main.goが起動を拒否する)。
	if cfg.DatabaseURL != "" || cfg.InternalAPIToken != "" {
		t.Errorf("DATABASE_URL and INTERNAL_API_TOKEN must default to empty: %+v", cfg)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PORT", "9001")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("INTERNAL_API_TOKEN", "shared-secret")

	cfg := Load()

	if cfg.Port != "9001" || cfg.DatabaseURL != "postgres://u:p@db:5432/x" || cfg.InternalAPIToken != "shared-secret" {
		t.Errorf("environment was not applied: %+v", cfg)
	}
}

func TestEmptyEnvironmentValueFallsBack(t *testing.T) {
	t.Setenv("PORT", "")
	if got := Load().Port; got != "8081" {
		t.Errorf("Port = %q, want the default", got)
	}
}
