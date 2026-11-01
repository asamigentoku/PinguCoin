package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/order-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"
)

type Server struct {
	pb.UnimplementedOrderServiceServer
	repo  *repository.Repository
	topic string
}

func New(repo *repository.Repository, topic string) *Server {
	return &Server{repo: repo, topic: topic}
}

func (s *Server) CreateOrder(ctx context.Context, in *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	key := strings.TrimSpace(in.GetIdempotencyKey())
	quantity := in.GetQuantity()
	if quantity <= 0 {
		quantity = 1
	}
	if in.GetUserId() == 0 || in.GetProductId() == 0 || key == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, product_id and idempotency_key are required")
	}
	if in.GetUnitPrice() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "unit_price must be greater than zero")
	}
	method := strings.TrimSpace(in.GetPaymentMethod())
	if method == "" {
		method = "point"
	}
	order, replayed, err := s.repo.CreateOrGet(&model.Order{UserID: uint(in.GetUserId()), ProductID: uint(in.GetProductId()), Quantity: quantity, UnitPrice: in.GetUnitPrice(), TotalAmount: in.GetUnitPrice() * quantity, Status: "pending", IdempotencyKey: key}, method, s.topic)
	if errors.Is(err, repository.ErrIdempotencyConflict) {
		return nil, status.Error(codes.AlreadyExists, "idempotency key was already used for another order")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not persist order")
	}
	if replayed {
		return &pb.CreateOrderResponse{Order: toPB(order), Replayed: true}, nil
	}
	return &pb.CreateOrderResponse{Order: toPB(order)}, nil
}
func (s *Server) GetOrder(_ context.Context, in *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	row, e := s.repo.Find(uint(in.GetId()), uint(in.GetUserId()))
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if e != nil {
		return nil, status.Error(codes.Internal, "order lookup failed")
	}
	return &pb.GetOrderResponse{Order: toPB(row)}, nil
}
func (s *Server) ListOrders(_ context.Context, in *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	rows, e := s.repo.List(uint(in.GetUserId()))
	if e != nil {
		return nil, status.Error(codes.Internal, "order lookup failed")
	}
	out := &pb.ListOrdersResponse{}
	for i := range rows {
		out.Orders = append(out.Orders, toPB(&rows[i]))
	}
	return out, nil
}
func (s *Server) HasPaidOrder(_ context.Context, in *pb.HasPaidOrderRequest) (*pb.HasPaidOrderResponse, error) {
	paid, e := s.repo.HasPaid(uint(in.GetUserId()), uint(in.GetProductId()))
	if e != nil {
		return nil, status.Error(codes.Internal, "order lookup failed")
	}
	return &pb.HasPaidOrderResponse{Paid: paid}, nil
}
func toPB(row *model.Order) *pb.Order {
	return &pb.Order{Id: uint64(row.ID), UserId: uint64(row.UserID), ProductId: uint64(row.ProductID), Quantity: row.Quantity, UnitPrice: row.UnitPrice, TotalAmount: row.TotalAmount, PaymentId: uint64(row.PaymentID), Status: row.Status, IdempotencyKey: row.IdempotencyKey, CreatedAt: timestamppb.New(row.CreatedAt), UpdatedAt: timestamppb.New(row.UpdatedAt)}
}
