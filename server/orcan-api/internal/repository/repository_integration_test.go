package repository_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/testutil"
)

// これらは実際のPostgresが必要(TEST_DATABASE_URL)。行ロックや制約など、DB自身の挙動を確かめる。

func newProduct(t *testing.T, db *gorm.DB, userID uint) *model.Product {
	t.Helper()
	product := &model.Product{UserID: userID, CategoryID: 1, Name: "Wallpaper", Price: 300}
	if err := repository.NewProductRepository(db).Create(product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	return product
}

func quantityOf(t *testing.T, repo *repository.ProductInventoryRepository, productID uint) int {
	t.Helper()
	inventory, err := repo.FindByProductID(productID)
	if err != nil {
		t.Fatalf("find inventory: %v", err)
	}
	return inventory.Quantity
}

// 商品を作ると、購入(在庫の消費)が通るように、在庫の行が同時に作られる。
func TestCreatingAProductCreatesItsInventory(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	product := newProduct(t, db, 1)

	inventory := repository.NewProductInventoryRepository(db)
	if got := quantityOf(t, inventory, product.ID); got != model.DefaultDigitalStock {
		t.Errorf("initial stock = %d, want %d", got, model.DefaultDigitalStock)
	}
}

func TestAdjustInventory(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	product := newProduct(t, db, 1)
	repo := repository.NewProductInventoryRepository(db)
	// 数を分かりやすくするため、在庫を10にしておく。
	inventory, _ := repo.FindByProductID(product.ID)
	inventory.Quantity = 10
	if err := repo.Update(inventory); err != nil {
		t.Fatal(err)
	}

	t.Run("consume", func(t *testing.T) {
		tx, replayed, err := repo.AdjustAtomic(product.ID, -3, "order", "key-consume")
		if err != nil || replayed {
			t.Fatalf("err=%v replayed=%v", err, replayed)
		}
		if tx.QuantityAfter != 7 || quantityOf(t, repo, product.ID) != 7 {
			t.Errorf("quantity after = %d, want 7", tx.QuantityAfter)
		}
	})

	t.Run("the same idempotency key does not consume twice", func(t *testing.T) {
		tx, replayed, err := repo.AdjustAtomic(product.ID, -3, "order", "key-consume")
		if err != nil {
			t.Fatal(err)
		}
		if !replayed {
			t.Error("a repeated key should be reported as replayed")
		}
		if tx.QuantityAfter != 7 || quantityOf(t, repo, product.ID) != 7 {
			t.Errorf("stock changed on replay: %d", quantityOf(t, repo, product.ID))
		}
	})

	t.Run("cannot consume more than the stock", func(t *testing.T) {
		_, _, err := repo.AdjustAtomic(product.ID, -8, "order", "key-too-many")
		if !errors.Is(err, repository.ErrInsufficientStock) {
			t.Fatalf("err = %v, want ErrInsufficientStock", err)
		}
		if got := quantityOf(t, repo, product.ID); got != 7 {
			t.Errorf("a rejected consume changed the stock to %d", got)
		}
	})

	t.Run("consuming exactly the remaining stock is allowed", func(t *testing.T) {
		if _, _, err := repo.AdjustAtomic(product.ID, -7, "order", "key-all"); err != nil {
			t.Fatal(err)
		}
		if got := quantityOf(t, repo, product.ID); got != 0 {
			t.Errorf("stock = %d, want 0", got)
		}
	})

	t.Run("release puts stock back", func(t *testing.T) {
		if _, _, err := repo.AdjustAtomic(product.ID, 2, "release", "key-release"); err != nil {
			t.Fatal(err)
		}
		if got := quantityOf(t, repo, product.ID); got != 2 {
			t.Errorf("stock = %d, want 2", got)
		}
	})

	t.Run("a product without an inventory row is an error", func(t *testing.T) {
		_, _, err := repo.AdjustAtomic(999999, -1, "order", "key-missing")
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("err = %v, want ErrRecordNotFound", err)
		}
	})
}

// 同時に購入されても、在庫より多くは売れない(行ロックが効いている)。
func TestConcurrentConsumeNeverOversells(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	product := newProduct(t, db, 1)
	repo := repository.NewProductInventoryRepository(db)
	inventory, _ := repo.FindByProductID(product.ID)
	inventory.Quantity = 5
	if err := repo.Update(inventory); err != nil {
		t.Fatal(err)
	}

	var succeeded, rejected atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := repo.AdjustAtomic(product.ID, -1, "order", "concurrent-"+string(rune('a'+i)))
			switch {
			case err == nil:
				succeeded.Add(1)
			case errors.Is(err, repository.ErrInsufficientStock):
				rejected.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if succeeded.Load() != 5 || rejected.Load() != 15 {
		t.Errorf("succeeded=%d rejected=%d, want 5 and 15", succeeded.Load(), rejected.Load())
	}
	if got := quantityOf(t, repo, product.ID); got != 0 {
		t.Errorf("final stock = %d, want 0", got)
	}
}

func TestFindAllFiltersBySeller(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	repo := repository.NewProductRepository(db)
	newProduct(t, db, 1)
	newProduct(t, db, 1)
	newProduct(t, db, 2)

	all, err := repo.FindAll(0)
	if err != nil || len(all) != 3 {
		t.Fatalf("FindAll(0) = %d products, err=%v; want 3", len(all), err)
	}
	mine, err := repo.FindAll(1)
	if err != nil || len(mine) != 2 {
		t.Fatalf("FindAll(1) = %d products, err=%v; want 2", len(mine), err)
	}
	for _, product := range mine {
		if product.UserID != 1 {
			t.Errorf("got a product of user %d", product.UserID)
		}
	}
}

func TestDeletedProductsAreHidden(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	repo := repository.NewProductRepository(db)
	product := newProduct(t, db, 1)

	if err := repo.Delete(product.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(product.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindByID after delete: err = %v, want ErrRecordNotFound", err)
	}
	if all, _ := repo.FindAll(0); len(all) != 0 {
		t.Errorf("FindAll returned %d products after delete", len(all))
	}
}
