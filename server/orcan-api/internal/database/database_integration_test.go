package database_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/testutil"
)

// これらは実際のPostgresが必要(TEST_DATABASE_URL)。起動時のマイグレーションが入れる初期データを確かめる。

func TestAutoMigrateSeedsTheCategoriesTheFrontendUses(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)

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

func TestAutoMigrateIsIdempotentAndKeepsEdits(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)

	if err := db.Model(&model.ProductCategory{}).Where("id = ?", 1).Update("name", "Renamed").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
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
	db := testutil.NewDB(t, database.AutoMigrate)

	category := model.ProductCategory{Name: "New category"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	if category.ID <= 4 {
		t.Errorf("new category got id %d, which collides with the seeded ids", category.ID)
	}
}

func TestAutoMigrateBackfillsInventoryForExistingProducts(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)

	// フックを使わずに作り、在庫の行が無い「既存の商品」を再現する。
	withoutInventory := &model.Product{UserID: 1, CategoryID: 1, Name: "Old product", Price: 100}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(withoutInventory).Error; err != nil {
		t.Fatal(err)
	}
	// すでに在庫がある商品は、上書きされてはいけない。
	withInventory := &model.Product{UserID: 1, CategoryID: 1, Name: "Stocked product", Price: 100}
	if err := db.Create(withInventory).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.ProductInventory{}).Where("product_id = ?", withInventory.ID).Update("quantity", 5).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
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
}

func TestAutoMigrateSeedsAssetPurposes(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)

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
