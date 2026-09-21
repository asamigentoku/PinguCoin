package grpcserver

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

// ProductListingServer は proto の `service ProductListingService` の実装。
// 1商品(product_id)に対して複数回の出品(ProductListing)を許容する想定
// (例: 一度出品終了し、再度別条件で出品し直すケース)。
type ProductListingServer struct {
	pb.UnimplementedProductListingServiceServer
	repo *repository.ProductListingRepository
}

func NewProductListingServer(repo *repository.ProductListingRepository) *ProductListingServer {
	return &ProductListingServer{repo: repo}
}

// ListProductListings は出品情報の一覧を返す。product_id を指定するとその商品の出品だけに絞り込む。
func (s *ProductListingServer) ListProductListings(ctx context.Context, req *pb.ListProductListingsRequest) (*pb.ListProductListingsResponse, error) {
	var productID uint
	if req.ProductId != nil {
		productID = uint(req.GetProductId())
	}

	listings, err := s.repo.FindAll(productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListProductListingsResponse{}
	for i := range listings {
		resp.Listings = append(resp.Listings, toProtoListing(&listings[i]))
	}
	return resp, nil
}

// GetProductListing はIDを1件指定して出品情報を取得する。
func (s *ProductListingServer) GetProductListing(ctx context.Context, req *pb.GetProductListingRequest) (*pb.GetProductListingResponse, error) {
	listing, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product listing", err)
	}
	return &pb.GetProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// CreateProductListing は新規出品を1件作成する。
// listed_at が未指定(nil)なら「今このリクエストが来た時刻」を出品日時として使う。
func (s *ProductListingServer) CreateProductListing(ctx context.Context, req *pb.CreateProductListingRequest) (*pb.CreateProductListingResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if req.GetPrice() < 0 {
		return nil, apperr.InvalidArgument("price must not be negative")
	}

	listedAt := time.Now()
	if req.GetListedAt() != nil {
		listedAt = req.GetListedAt().AsTime()
	}

	listing := &model.ProductListing{
		ProductID: uint(req.GetProductId()),
		Price:     req.GetPrice(),
		Status:    req.GetStatus(),
		ListedAt:  listedAt,
	}

	if err := s.repo.Create(listing); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// UpdateProductListing は既存の出品情報を更新する。
// ended_at を指定すると出品終了日時をセットし、未指定に戻すとnil(出品中)に戻す。
func (s *ProductListingServer) UpdateProductListing(ctx context.Context, req *pb.UpdateProductListingRequest) (*pb.UpdateProductListingResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if req.GetPrice() < 0 {
		return nil, apperr.InvalidArgument("price must not be negative")
	}

	listing, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product listing", err)
	}

	listing.ProductID = uint(req.GetProductId())
	listing.Price = req.GetPrice()
	listing.Status = req.GetStatus()
	if req.GetListedAt() != nil {
		listing.ListedAt = req.GetListedAt().AsTime()
	}
	if req.GetEndedAt() != nil {
		endedAt := req.GetEndedAt().AsTime()
		listing.EndedAt = &endedAt
	} else {
		listing.EndedAt = nil
	}

	if err := s.repo.Update(listing); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// DeleteProductListing はIDを指定して出品情報を削除する。
func (s *ProductListingServer) DeleteProductListing(ctx context.Context, req *pb.DeleteProductListingRequest) (*pb.DeleteProductListingResponse, error) {
	if err := s.repo.Delete(uint(req.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductListingResponse{}, nil
}

// toProtoListing はDBのmodel.ProductListingをレスポンス用のpb.ProductListingに変換する。
// EndedAt は *time.Time (未終了ならnil) なので、nilなら変換自体をスキップしてprotoのフィールドもnilにする。
func toProtoListing(l *model.ProductListing) *pb.ProductListing {
	p := &pb.ProductListing{
		Id:        uint32(l.ID),
		ProductId: uint32(l.ProductID),
		Price:     l.Price,
		Status:    l.Status,
		ListedAt:  timestamppb.New(l.ListedAt),
		CreatedAt: timestamppb.New(l.CreatedAt),
		UpdatedAt: timestamppb.New(l.UpdatedAt),
	}
	if l.EndedAt != nil {
		p.EndedAt = timestamppb.New(*l.EndedAt)
	}
	return p
}
