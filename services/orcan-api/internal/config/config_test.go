package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"INTERNAL_API_TOKEN", "AZURE_STORAGE_CONNECTION_STRING", "AZURE_STORAGE_PUBLIC_CONTAINER", "AZURE_STORAGE_PRIVATE_CONTAINER", "APP_ENV"} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.Port != "8080" || cfg.DBHost != "localhost" || cfg.DBPort != "5432" || cfg.DBName != "orcan" || cfg.DBSSLMode != "disable" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.DatabaseURL != "" || cfg.InternalAPIToken != "" {
		t.Errorf("DATABASE_URL and INTERNAL_API_TOKEN must default to empty: %+v", cfg)
	}
	if cfg.AzureStoragePublicContainer != "pingue-public" || cfg.AzureStoragePrivateContainer != "pingue" || cfg.AppEnv != "develop" {
		t.Errorf("unexpected storage defaults: %+v", cfg)
	}
	if cfg.AzureStorageConnectionString == "" {
		t.Error("the connection string should default to the local Azurite emulator")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("INTERNAL_API_TOKEN", "shared-secret")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AZURE_STORAGE_PRIVATE_CONTAINER", "private-files")

	cfg := Load()

	if cfg.Port != "9000" || cfg.DatabaseURL != "postgres://u:p@db:5432/x" || cfg.InternalAPIToken != "shared-secret" ||
		cfg.AppEnv != "staging" || cfg.AzureStoragePrivateContainer != "private-files" {
		t.Errorf("environment was not applied: %+v", cfg)
	}
}

// 空文字の環境変数は「未設定」と同じ扱い(デフォルトに戻る)。
func TestEmptyEnvironmentValueFallsBack(t *testing.T) {
	t.Setenv("PORT", "")
	if got := Load().Port; got != "8080" {
		t.Errorf("Port = %q, want the default", got)
	}
}
