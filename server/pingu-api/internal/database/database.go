package database

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/config"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/model"
)

// Connect はPostgreSQLへ接続し、*gorm.DBを返す。
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

// AutoMigrate はpingu-apiが扱う全モデル(Order)のマイグレーションを実行する。
// 商品(Product)・ユーザー(User)はorcan-apiが真実の記録を持つため、ここでは扱わない。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Order{},
	)
}
