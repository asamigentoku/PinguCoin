package grpcserver

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
)

// 実際のPostgresが必要(TEST_DATABASE_URL)。注文APIが頼りにする、在庫APIのエラーコードを確かめる。
func TestAdjustProductInventoryOverGRPC(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)
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

// 在庫の増減が、方向(increase / decrease)と結果(succeeded / replayed / insufficient)で、メトリクスに数えられる。
// 在庫不足は、サーバーの失敗(failed)ではなく、insufficient として数える(failed は、アラートの対象なので、混ぜない)。
func TestInventoryMetrics(t *testing.T) {
	value := func(direction, result string) float64 {
		families, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, family := range families {
			if family.GetName() != "pingucoin_inventory_adjustments_total" {
				continue
			}
			for _, metric := range family.GetMetric() {
				labels := map[string]string{}
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				if labels["direction"] == direction && labels["result"] == result {
					return metric.GetCounter().GetValue()
				}
			}
		}
		return 0
	}
	decreased, replayed, insufficient, increased := value("decrease", "succeeded"), value("decrease", "replayed"), value("decrease", "insufficient"), value("increase", "succeeded")

	db := testutil.NewDB(t, database.Migrate)
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
	adjust := func(amount int32, key string) {
		_, _ = server.AdjustProductInventory(context.Background(), &pb.AdjustProductInventoryRequest{ProductId: uint32(product.ID), Amount: amount, IdempotencyKey: key})
	}

	adjust(-1, "m1")
	adjust(-1, "m1") // 再送
	adjust(-9, "m2") // 在庫不足
	adjust(3, "m3")  // 戻し

	for _, check := range []struct {
		name        string
		got, before float64
	}{
		{"decrease succeeded", value("decrease", "succeeded"), decreased},
		{"decrease replayed", value("decrease", "replayed"), replayed},
		{"decrease insufficient", value("decrease", "insufficient"), insufficient},
		{"increase succeeded", value("increase", "succeeded"), increased},
	} {
		if got := check.got - check.before; got != 1 {
			t.Errorf("%s increased by %v, want 1", check.name, got)
		}
	}
}
