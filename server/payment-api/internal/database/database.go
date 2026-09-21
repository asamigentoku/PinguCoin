package database

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/config"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
)

// Connect はPostgreSQLへ接続し、*gorm.DBを返す。
// logger にはmain.goで設定したslog.Loggerを渡し、GORM自身のクエリログも
// アプリ全体と同じ構造化ログ(JSON)に統一する。
func Connect(cfg config.Config, logger *slog.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: NewGormLogger(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	return db, nil
}

// AutoMigrate はpayment-apiが扱う全モデルのマイグレーションを実行する。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Payment{},
		&model.Refund{},
		&model.PointAccount{},
		&model.PointTransaction{},
	)
}
