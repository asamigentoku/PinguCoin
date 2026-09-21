package config

import "os"

// Config はアプリケーションの設定値を保持する。
type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	OrcanAddr   string
	PaymentAddr string

	// ClerkSecretKey はClerkのBackend API(JWKS取得・ユーザー情報取得)を呼ぶためのSecret Key。
	ClerkSecretKey string
}

// Load は環境変数から設定を読み込む。未設定の項目にはデフォルト値を使う。
func Load() Config {
	return Config{
		Port:       getEnv("PORT", "8082"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "pingu"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		OrcanAddr:   getEnv("ORCAN_API_ADDR", "localhost:8080"),
		PaymentAddr: getEnv("PAYMENT_API_ADDR", "localhost:8081"),

		ClerkSecretKey: getEnv("CLERK_SECRET_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
