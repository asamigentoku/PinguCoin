package database

import (
	"context"
	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/services/order-api/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log/slog"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
}
func Migrate(db *gorm.DB) error {
	list, e := dbmigrate.Load(migrations.FS, ".")
	if e != nil {
		return e
	}
	_, e = dbmigrate.Run(context.Background(), db, list, dbmigrate.Options{BaselineVersion: 1, SentinelTable: "orders", Table: "order_schema_migrations", Logger: slog.Default()})
	return e
}
