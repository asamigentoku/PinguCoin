package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

type Server struct {
	pb.UnimplementedProductInventoryServiceServer
	repo *repository.InventoryRepository
}

func New(repo *repository.InventoryRepository) *Server { return &Server{repo: repo} }

func (s *Server) ListProductInventories(context.Context, *pb.ListProductInventoriesRequest) (*pb.ListProductInventoriesResponse, error) {
	rows, err := s.repo.FindAll()
	if err != nil {
		return nil, status.Error(codes.Internal, "inventory lookup failed")
	}
	response := &pb.ListProductInventoriesResponse{}
	for i := range rows {
		response.Inventories = append(response.Inventories, toPB(&rows[i]))
	}
	return response, nil
}
func (s *Server) GetProductInventory(_ context.Context, in *pb.GetProductInventoryRequest) (*pb.GetProductInventoryResponse, error) {
	row, err := s.repo.FindByID(uint(in.GetId()))
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetProductInventoryResponse{Inventory: toPB(row)}, nil
}
func (s *Server) CreateProductInventory(_ context.Context, in *pb.CreateProductInventoryRequest) (*pb.CreateProductInventoryResponse, error) {
	if in.GetProductId() == 0 || in.GetQuantity() < 0 || in.GetReserved() < 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid inventory")
	}
	row := &model.Inventory{ProductID: uint(in.GetProductId()), Quantity: int(in.GetQuantity()), Reserved: int(in.GetReserved())}
	if err := s.repo.Create(row); err != nil {
		return nil, mapError(err)
	}
	return &pb.CreateProductInventoryResponse{Inventory: toPB(row)}, nil
}
func (s *Server) UpdateProductInventory(_ context.Context, in *pb.UpdateProductInventoryRequest) (*pb.UpdateProductInventoryResponse, error) {
	if in.GetQuantity() < 0 || in.GetReserved() < 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid inventory")
	}
	row, err := s.repo.FindByID(uint(in.GetId()))
	if err != nil {
		return nil, mapError(err)
	}
	row.ProductID, row.Quantity, row.Reserved = uint(in.GetProductId()), int(in.GetQuantity()), int(in.GetReserved())
	if err = s.repo.Update(row); err != nil {
		return nil, mapError(err)
	}
	return &pb.UpdateProductInventoryResponse{Inventory: toPB(row)}, nil
}
func (s *Server) DeleteProductInventory(_ context.Context, in *pb.DeleteProductInventoryRequest) (*pb.DeleteProductInventoryResponse, error) {
	if err := s.repo.Delete(uint(in.GetId())); err != nil {
		return nil, mapError(err)
	}
	return &pb.DeleteProductInventoryResponse{}, nil
}
func (s *Server) AdjustProductInventory(_ context.Context, in *pb.AdjustProductInventoryRequest) (*pb.AdjustProductInventoryResponse, error) {
	key := strings.TrimSpace(in.GetIdempotencyKey())
	if in.GetProductId() == 0 || in.GetAmount() == 0 || key == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id, amount and idempotency_key are required")
	}
	row, _, err := s.repo.Adjust(uint(in.GetProductId()), int(in.GetAmount()), in.GetReason(), key)
	if errors.Is(err, repository.ErrInsufficientStock) {
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	}
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.AdjustProductInventoryResponse{Inventory: toPB(row)}, nil
}
func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "inventory not found")
	}
	return status.Error(codes.Internal, "inventory operation failed")
}

func toPB(row *model.Inventory) *pb.ProductInventory {
	return &pb.ProductInventory{Id: uint32(row.ID), ProductId: uint32(row.ProductID), Quantity: int32(row.Quantity), Reserved: int32(row.Reserved), Version: uint32(row.Version), CreatedAt: timestamppb.New(row.CreatedAt), UpdatedAt: timestamppb.New(row.UpdatedAt)}
}
