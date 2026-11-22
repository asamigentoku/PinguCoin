package config

import (
	"os"
	"strings"
)

// Config はorder-apiの起動に必要な外部接続先と認証設定です。
type Config struct {
	DatabaseURL      string
	Port             string
	InternalAPIToken string
	KafkaBrokers     []string
	KafkaTopic       string
}

func Load() Config {
	// Kafkaブローカーはカンマ区切りで受け取り、空要素は除外します。
	var brokers []string
	for _, broker := range strings.Split(os.Getenv("KAFKA_BROKERS"), ",") {
		if value := strings.TrimSpace(broker); value != "" {
			brokers = append(brokers, value)
		}
	}
	return Config{DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable"), Port: env("PORT", "8084"), InternalAPIToken: os.Getenv("INTERNAL_API_TOKEN"), KafkaBrokers: brokers, KafkaTopic: env("KAFKA_ORDER_TOPIC", "orders.created.v1")}
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
