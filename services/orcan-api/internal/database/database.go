package database

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/pkg/gormlogger"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/migrations"
)

// Connect はPostgreSQLへ接続し、*gorm.DBを返す。
// logger にはmain.goで設定したslog.Loggerを渡し、GORM自身のクエリログも
// アプリ全体と同じ構造化ログ(JSON)に統一する。
func Connect(cfg config.Config, logger *slog.Logger) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.New(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	return db, nil
}

// Migrate は、orcan-api の DB スキーマを、最新にする。起動時に1回だけ呼ぶ。
//
// バージョン付きの SQL ファイル(services/orcan-api/migrations)のうち、まだ適用していないものを、番号順に適用する
// (仕組みと、ルールは pkg/dbmigrate を参照)。
//
// 以前(gorm の AutoMigrate)に作られた既存の DB では、番号 1 のファイル(導入した時点のスキーマ全体)を、実行せずに
// 「適用済み」と記録する。products テーブルがあれば、既存の DB と判断する。
func Migrate(db *gorm.DB) error {
	migrationList, err := dbmigrate.Load(migrations.FS, ".")
	if err != nil {
		return err
	}
	_, err = dbmigrate.Run(context.Background(), db, migrationList, dbmigrate.Options{
		BaselineVersion: 1,
		SentinelTable:   "products",
		// 記録のテーブルは、サービスごとに別にする(開発・staging では、3つのサービスが、同じ DB を共有するため)。
		Table:  "orcan_schema_migrations",
		Logger: slog.Default(),
	})
	return err
}
