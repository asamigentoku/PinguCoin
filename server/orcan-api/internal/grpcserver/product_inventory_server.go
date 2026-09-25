package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

// ProductInventoryServer は proto の `service ProductInventoryService` の実装。
// 1商品(product_id)につき在庫レコードは1件の想定(quantity=在庫数, reserved=引当済み数)。
type ProductInventoryServer struct {
	pb.UnimplementedProductInventoryServiceServer
	repo *repository.ProductInventoryRepository
}

func NewProductInventoryServer(repo *repository.ProductInventoryRepository) *ProductInventoryServer {
	return &ProductInventoryServer{repo: repo}
}

// ListProductInventories は在庫一覧を返す(絞り込みなし、全件)。
func (server *ProductInventoryServer) ListProductInventories(ctx context.Context, request *pb.ListProductInventoriesRequest) (*pb.ListProductInventoriesResponse, error) {
	inventories, err := server.repo.FindAll()
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListProductInventoriesResponse{}
	for i := range inventories {
		response.Inventories = append(response.Inventories, toProtoInventory(&inventories[i]))
	}
	return response, nil
}

func (server *ProductInventoryServer) GetProductInventory(ctx context.Context, request *pb.GetProductInventoryRequest) (*pb.GetProductInventoryResponse, error) {
	inventory, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product inventory", err)
	}
	return &pb.GetProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (server *ProductInventoryServer) CreateProductInventory(ctx context.Context, request *pb.CreateProductInventoryRequest) (*pb.CreateProductInventoryResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetQuantity() < 0 || request.GetReserved() < 0 {
		return nil, apperr.InvalidArgument("quantity and reserved must not be negative")
	}

	inventory := &model.ProductInventory{
		ProductID: uint(request.GetProductId()),
		Quantity:  int(request.GetQuantity()),
		Reserved:  int(request.GetReserved()),
	}

	if err := server.repo.Create(inventory); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (server *ProductInventoryServer) UpdateProductInventory(ctx context.Context, request *pb.UpdateProductInventoryRequest) (*pb.UpdateProductInventoryResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetQuantity() < 0 || request.GetReserved() < 0 {
		return nil, apperr.InvalidArgument("quantity and reserved must not be negative")
	}

	inventory, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product inventory", err)
	}

	inventory.ProductID = uint(request.GetProductId())
	inventory.Quantity = int(request.GetQuantity())
	inventory.Reserved = int(request.GetReserved())

	if err := server.repo.Update(inventory); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (server *ProductInventoryServer) DeleteProductInventory(ctx context.Context, request *pb.DeleteProductInventoryRequest) (*pb.DeleteProductInventoryResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductInventoryResponse{}, nil
}

// AdjustProductInventory は在庫数(quantity)をamountだけ増減させる(負=消費、正=戻し)。
// idempotency_keyで冪等性を担保する(同じキーの再送は二重に増減させない)。
func (server *ProductInventoryServer) AdjustProductInventory(ctx context.Context, request *pb.AdjustProductInventoryRequest) (*pb.AdjustProductInventoryResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetAmount() == 0 {
		return nil, apperr.InvalidArgument("amount must not be zero")
	}
	idempotencyKey := strings.TrimSpace(request.GetIdempotencyKey())
	if idempotencyKey == "" {
		return nil, apperr.InvalidArgument("idempotency_key is required")
	}

	_, _, err := server.repo.AdjustAtomic(uint(request.GetProductId()), int(request.GetAmount()), request.GetReason(), idempotencyKey)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			return nil, apperr.FailedPrecondition("insufficient stock")
		}
		return nil, apperr.Internal(err)
	}

	inventory, err := server.repo.FindByProductID(uint(request.GetProductId()))
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.AdjustProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

// toProtoInventory はDBのmodel.ProductInventoryをレスポンス用のpb.ProductInventoryに変換する。
func toProtoInventory(inventory *model.ProductInventory) *pb.ProductInventory {
	return &pb.ProductInventory{
		Id:        uint32(inventory.ID),
		ProductId: uint32(inventory.ProductID),
		Quantity:  int32(inventory.Quantity),
		Reserved:  int32(inventory.Reserved),
		CreatedAt: timestamppb.New(inventory.CreatedAt),
		UpdatedAt: timestamppb.New(inventory.UpdatedAt),
		Version:   uint32(inventory.Version),
	}
}
