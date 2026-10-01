package database_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/migrations"
)

// これらは実際のPostgresが必要(TEST_DATABASE_URL)。起動時のマイグレーションが入れる初期データを確かめる。

func TestMigrateSeedsTheCategoriesTheFrontendUses(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)

	// client-web の lib/categories.ts と同じID・名前。無いと、出品時に外部キー制約違反になる。
	want := map[uint]string{1: "アート・イラスト", 2: "テンプレート", 3: "音楽・サウンド", 4: "便利ツール"}
	var categories []model.ProductCategory
	if err := db.Order("id").Find(&categories).Error; err != nil {
		t.Fatal(err)
	}
	if len(categories) != len(want) {
		t.Fatalf("got %d categories, want %d", len(categories), len(want))
	}
	for _, category := range categories {
		if want[category.ID] != category.Name {
			t.Errorf("category %d = %q, want %q", category.ID, category.Name, want[category.ID])
		}
	}
}

func TestMigrateIsIdempotentAndKeepsEdits(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)

	if err := db.Model(&model.ProductCategory{}).Where("id = ?", 1).Update("name", "Renamed").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	var category model.ProductCategory
	if err := db.First(&category, 1).Error; err != nil {
		t.Fatal(err)
	}
	if category.Name != "Renamed" {
		t.Errorf("a re-run overwrote the category name: %q", category.Name)
	}
	var count int64
	db.Model(&model.ProductCategory{}).Count(&count)
	if count != 4 {
		t.Errorf("categories after re-run = %d, want 4", count)
	}
}

// 新しく作るカテゴリーのIDが、初期データ(1〜4)と衝突しない(シーケンスが進んでいる)。
func TestNewCategoriesDoNotCollideWithTheSeed(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)

	category := model.ProductCategory{Name: "New category"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	if category.ID <= 4 {
		t.Errorf("new category got id %d, which collides with the seeded ids", category.ID)
	}
}

// 既存の DB(データがある)を、最新のスキーマに上げる: 在庫の補完、旧データの移行、旧カラムの削除。
// 「ベースラインと参照データまでを適用した状態」を作って、そこへ、残りのマイグレーションを適用して確かめる。
func TestUpgradingAnExistingDatabase(t *testing.T) {
	db := testutil.NewDB(t, nil)
	ctx := context.Background()
	all, err := dbmigrate.Load(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbmigrate.Run(ctx, db, all[:2], dbmigrate.Options{Table: "orcan_schema_migrations"}); err != nil { // 1: ベースライン、2: 参照データ
		t.Fatal(err)
	}

	// 在庫の行が無い既存の商品(画像・ファイルの参照つき)、すでに在庫がある商品、旧カラム(password_hash)がある users。
	withoutInventory := &model.Product{UserID: 1, CategoryID: 1, Name: "Old product", Price: 100, ImageURL: "https://blob/old.png", FileURL: "https://blob/old.zip"}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(withoutInventory).Error; err != nil {
		t.Fatal(err)
	}
	withInventory := &model.Product{UserID: 1, CategoryID: 1, Name: "Stocked product", Price: 100}
	if err := db.Create(withInventory).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.ProductInventory{}).Where("product_id = ?", withInventory.ID).Update("quantity", 5).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE users ADD COLUMN password_hash text NOT NULL DEFAULT ''").Error; err != nil {
		t.Fatal(err)
	}

	applied, err := dbmigrate.Run(ctx, db, all, dbmigrate.Options{Table: "orcan_schema_migrations"})
	if err != nil || len(applied) != len(all)-2 {
		t.Fatalf("applied=%v err=%v, want the remaining %d migrations", applied, err, len(all)-2)
	}

	quantity := func(productID uint) int {
		var inventory model.ProductInventory
		if err := db.Where("product_id = ?", productID).First(&inventory).Error; err != nil {
			t.Fatalf("no inventory for product %d: %v", productID, err)
		}
		return inventory.Quantity
	}
	if got := quantity(withoutInventory.ID); got != model.DefaultDigitalStock {
		t.Errorf("backfilled stock = %d, want %d", got, model.DefaultDigitalStock)
	}
	if got := quantity(withInventory.ID); got != 5 {
		t.Errorf("existing stock was overwritten: %d", got)
	}

	// 旧カラム(image_url / file_url)の参照が、共通のテーブルにコピーされる(画像は用途1・メイン、ファイルは用途3)。
	var assets []model.ProductAsset
	if err := db.Where("product_id = ?", withoutInventory.ID).Order("purpose_id").Find(&assets).Error; err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].PurposeID != 1 || !assets[0].IsPrimary || assets[0].StorageURL != "https://blob/old.png" || assets[1].PurposeID != 3 {
		t.Errorf("legacy references were not migrated: %+v", assets)
	}

	if db.Migrator().HasColumn(&model.User{}, "password_hash") {
		t.Error("users.password_hash should have been dropped")
	}
}

func TestMigrateSeedsAssetPurposes(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)

	var purposes []model.ProductAssetPurpose
	if err := db.Order("id").Find(&purposes).Error; err != nil {
		t.Fatal(err)
	}
	if len(purposes) != 3 {
		t.Fatalf("got %d purposes, want 3", len(purposes))
	}
	// 販売するファイルは非公開、画像は公開。
	for _, purpose := range purposes {
		wantPublic := purpose.ID != model.ProductAssetPurposeProductFile
		if purpose.IsPublic != wantPublic {
			t.Errorf("purpose %d (%s): is_public = %v, want %v", purpose.ID, purpose.Name, purpose.IsPublic, wantPublic)
		}
	}
}
