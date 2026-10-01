package database

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/pkg/gormlogger"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
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

// AutoMigrate はorcan-apiが扱う全モデルのマイグレーションを実行する。
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.ProductCategory{},
		&model.Product{},
		&model.ProductAssetPurpose{},
		&model.ProductAsset{},
		&model.ProductDetail{},
		&model.ProductInventory{},
		&model.ProductInventoryTransaction{},
		&model.ProductListing{},
		&model.User{},
	); err != nil {
		return err
	}

	// Purpose is data, not a database enum/check constraint. Additional purposes
	// can therefore be added later without changing the product_assets schema.
	defaultPurposes := []model.ProductAssetPurpose{
		{ID: model.ProductAssetPurposeProductImage, Name: "product_image", IsPublic: true},
		{ID: model.ProductAssetPurposeDetailImage, Name: "detail_image", IsPublic: true},
		{ID: model.ProductAssetPurposeProductFile, Name: "product_file", IsPublic: false},
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaultPurposes).Error; err != nil {
		return err
	}
	// client-web の lib/categories.ts と同じID・名前。商品はcategory_idの外部キーを持つため、
	// 行が無いと出品(商品の作成)が制約違反で失敗する。既にある行は変更しない。
	defaultCategories := []model.ProductCategory{
		{ID: 1, Name: "アート・イラスト"},
		{ID: 2, Name: "テンプレート"},
		{ID: 3, Name: "音楽・サウンド"},
		{ID: 4, Name: "便利ツール"},
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaultCategories).Error; err != nil {
		return err
	}
	// IDを明示して入れるとシーケンスが進まないため、以降にCreateProductCategoryで
	// 採番するIDと衝突しないよう、シーケンスを最大IDまで進める。
	if err := db.Exec(`SELECT setval(pg_get_serial_sequence('product_categories', 'id'), (SELECT COALESCE(MAX(id), 1) FROM product_categories))`).Error; err != nil {
		return err
	}
	// 在庫の行が無い既存の商品にも、購入できるよう初期在庫を入れる。
	if err := db.Exec(`INSERT INTO product_inventory (product_id, quantity, reserved, version, created_at, updated_at)
		SELECT id, ?, 0, 1, NOW(), NOW() FROM products WHERE deleted_at IS NULL
		ON CONFLICT (product_id) DO NOTHING`, model.DefaultDigitalStock).Error; err != nil {
		return err
	}
	if err := migrateLegacyProductAssets(db); err != nil {
		return err
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_assets_one_primary
		ON product_assets (product_id, purpose_id) WHERE is_primary = TRUE`).Error; err != nil {
		return err
	}

	// AutoMigrateはカラムの追加のみ行い削除はしないため、認証をClerkへ移行したことで
	// 不要になった旧カラム(平文パスワードのハッシュ)は明示的に落とす。
	// 残したままだとNOT NULL制約により新規ユーザー作成(clerk_user_idのみ指定)が失敗する。
	if db.Migrator().HasColumn(&model.User{}, "password_hash") {
		if err := db.Migrator().DropColumn(&model.User{}, "password_hash"); err != nil {
			return err
		}
	}

	return nil
}

// migrateLegacyProductAssets copies references from the former three storage
// shapes into the common table. ON CONFLICT makes startup migration idempotent.
func migrateLegacyProductAssets(db *gorm.DB) error {
	statements := []string{
		`INSERT INTO product_assets
			(product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
		 SELECT id, 1, image_url, '', '', 0, '', 0, TRUE, '{}'::jsonb, created_at, updated_at
		 FROM products WHERE image_url IS NOT NULL AND image_url <> ''
		 ON CONFLICT (storage_url) DO NOTHING`,
		`INSERT INTO product_assets
			(product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
		 SELECT product_id, 2, image_url, '', '', 0, description, sort_order, FALSE, '{}'::jsonb, created_at, updated_at
		 FROM product_detail WHERE image_url IS NOT NULL AND image_url <> ''
		 ON CONFLICT (storage_url) DO NOTHING`,
		`INSERT INTO product_assets
			(product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
		 SELECT id, 3, file_url, '', '', 0, '', 0, FALSE, '{}'::jsonb, created_at, updated_at
		 FROM products WHERE file_url IS NOT NULL AND file_url <> ''
		 ON CONFLICT (storage_url) DO NOTHING`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
