package grpcserver

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/testutil"
)

// 実際のPostgresが必要(TEST_DATABASE_URL)。注文APIが頼りにする、在庫APIのエラーコードを確かめる。
func TestAdjustProductInventoryOverGRPC(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	product := &model.Product{UserID: 1, CategoryID: 1, Name: "Wallpaper", Price: 300}
	if err := repository.NewProductRepository(db).Create(product); err != nil {
		t.Fatal(err)
	}
	inventoryRepo := repository.NewProductInventoryRepository(db)
	inventory, _ := inventoryRepo.FindByProductID(product.ID)
	inventory.Quantity = 2
	if err := inventoryRepo.Update(inventory); err != nil {
		t.Fatal(err)
	}
	server := NewProductInventoryServer(inventoryRepo)
	ctx := context.Background()

	adjust := func(amount int32, key string) (*pb.AdjustProductInventoryResponse, error) {
		return server.AdjustProductInventory(ctx, &pb.AdjustProductInventoryRequest{ProductId: uint32(product.ID), Amount: amount, IdempotencyKey: key})
	}

	response, err := adjust(-1, "k1")
	if err != nil || response.GetInventory().GetQuantity() != 1 {
		t.Fatalf("consume 1: quantity=%d err=%v", response.GetInventory().GetQuantity(), err)
	}

	// 同じキーの再送は、二重に減らさず同じ結果を返す。
	response, err = adjust(-1, "k1")
	if err != nil || response.GetInventory().GetQuantity() != 1 {
		t.Fatalf("replay: quantity=%d err=%v", response.GetInventory().GetQuantity(), err)
	}

	// 在庫不足は FailedPrecondition。注文APIはこれを見て、決済に進まずに打ち切る。
	if _, err := adjust(-5, "k2"); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("insufficient stock: code = %v, want FailedPrecondition", status.Code(err))
	}
}
