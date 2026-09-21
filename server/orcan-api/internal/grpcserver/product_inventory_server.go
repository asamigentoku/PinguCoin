package grpcserver

import (
	"context"

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
func (s *ProductInventoryServer) ListProductInventories(ctx context.Context, req *pb.ListProductInventoriesRequest) (*pb.ListProductInventoriesResponse, error) {
	inventories, err := s.repo.FindAll()
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListProductInventoriesResponse{}
	for i := range inventories {
		resp.Inventories = append(resp.Inventories, toProtoInventory(&inventories[i]))
	}
	return resp, nil
}

func (s *ProductInventoryServer) GetProductInventory(ctx context.Context, req *pb.GetProductInventoryRequest) (*pb.GetProductInventoryResponse, error) {
	inventory, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product inventory", err)
	}
	return &pb.GetProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (s *ProductInventoryServer) CreateProductInventory(ctx context.Context, req *pb.CreateProductInventoryRequest) (*pb.CreateProductInventoryResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if req.GetQuantity() < 0 || req.GetReserved() < 0 {
		return nil, apperr.InvalidArgument("quantity and reserved must not be negative")
	}

	inventory := &model.ProductInventory{
		ProductID: uint(req.GetProductId()),
		Quantity:  int(req.GetQuantity()),
		Reserved:  int(req.GetReserved()),
	}

	if err := s.repo.Create(inventory); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (s *ProductInventoryServer) UpdateProductInventory(ctx context.Context, req *pb.UpdateProductInventoryRequest) (*pb.UpdateProductInventoryResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if req.GetQuantity() < 0 || req.GetReserved() < 0 {
		return nil, apperr.InvalidArgument("quantity and reserved must not be negative")
	}

	inventory, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product inventory", err)
	}

	inventory.ProductID = uint(req.GetProductId())
	inventory.Quantity = int(req.GetQuantity())
	inventory.Reserved = int(req.GetReserved())

	if err := s.repo.Update(inventory); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductInventoryResponse{Inventory: toProtoInventory(inventory)}, nil
}

func (s *ProductInventoryServer) DeleteProductInventory(ctx context.Context, req *pb.DeleteProductInventoryRequest) (*pb.DeleteProductInventoryResponse, error) {
	if err := s.repo.Delete(uint(req.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductInventoryResponse{}, nil
}

// toProtoInventory はDBのmodel.ProductInventoryをレスポンス用のpb.ProductInventoryに変換する。
func toProtoInventory(i *model.ProductInventory) *pb.ProductInventory {
	return &pb.ProductInventory{
		Id:        uint32(i.ID),
		ProductId: uint32(i.ProductID),
		Quantity:  int32(i.Quantity),
		Reserved:  int32(i.Reserved),
		CreatedAt: timestamppb.New(i.CreatedAt),
		UpdatedAt: timestamppb.New(i.UpdatedAt),
	}
}
