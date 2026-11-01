package database

import (
	"context"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/migrations"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
}
func Migrate(db *gorm.DB) error {
	list, err := dbmigrate.Load(migrations.FS, ".")
	if err != nil {
		return err
	}
	_, err = dbmigrate.Run(context.Background(), db, list, dbmigrate.Options{BaselineVersion: 1, SentinelTable: "product_inventory", Table: "inventory_schema_migrations", Logger: slog.Default()})
	return err
}
