package config

import (
	"os"
	"strings"
)

// Config はアプリケーションの設定値を保持する。
type Config struct {
	DatabaseURL string
	Port        string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string

	// InternalAPIToken はbffからのgRPC呼び出しを検証するための共有シークレット。
	// bff/media-api/payment-apiで同じ値を設定する。
	InternalAPIToken string
	KafkaBrokers     []string
	MockStripeURL    string
	WebhookPort      string
	WebhookSecret    string
	PaymentReturnURL string
}

// Load は環境変数から設定を読み込む。未設定の項目にはデフォルト値を使う。

func Load() Config {
	var brokers []string
	for _, broker := range strings.Split(os.Getenv("KAFKA_BROKERS"), ",") {
		if value := strings.TrimSpace(broker); value != "" {
			brokers = append(brokers, value)
		}
	}
	return Config{
		DatabaseURL: getEnv("DATABASE_URL", ""),
		Port:        getEnv("PORT", "8081"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", "postgres"),
		DBName:      getEnv("DB_NAME", "payment"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),

		InternalAPIToken: getEnv("INTERNAL_API_TOKEN", ""),
		KafkaBrokers:     brokers,
		MockStripeURL:    getEnv("MOCK_STRIPE_URL", "http://localhost:8090"),
		WebhookPort:      getEnv("WEBHOOK_PORT", "8086"),
		WebhookSecret:    getEnv("WEBHOOK_SECRET", "local-webhook-secret"),
		PaymentReturnURL: getEnv("PAYMENT_RETURN_URL", "http://localhost:3000/purchases"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
