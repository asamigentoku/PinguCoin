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
func (server *ProductListingServer) ListProductListings(ctx context.Context, request *pb.ListProductListingsRequest) (*pb.ListProductListingsResponse, error) {
	var productID uint
	if request.ProductId != nil {
		productID = uint(request.GetProductId())
	}

	listings, err := server.repo.FindAll(productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListProductListingsResponse{}
	for i := range listings {
		response.Listings = append(response.Listings, toProtoListing(&listings[i]))
	}
	return response, nil
}

// GetProductListing はIDを1件指定して出品情報を取得する。
func (server *ProductListingServer) GetProductListing(ctx context.Context, request *pb.GetProductListingRequest) (*pb.GetProductListingResponse, error) {
	listing, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product listing", err)
	}
	return &pb.GetProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// CreateProductListing は新規出品を1件作成する。
// listed_at が未指定(nil)なら「今このリクエストが来た時刻」を出品日時として使う。
func (server *ProductListingServer) CreateProductListing(ctx context.Context, request *pb.CreateProductListingRequest) (*pb.CreateProductListingResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetPrice() < 0 {
		return nil, apperr.InvalidArgument("price must not be negative")
	}

	listedAt := time.Now()
	if request.GetListedAt() != nil {
		listedAt = request.GetListedAt().AsTime()
	}

	listing := &model.ProductListing{
		ProductID: uint(request.GetProductId()),
		Price:     request.GetPrice(),
		Status:    request.GetStatus(),
		ListedAt:  listedAt,
	}

	if err := server.repo.Create(listing); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// UpdateProductListing は既存の出品情報を更新する。
// ended_at を指定すると出品終了日時をセットし、未指定に戻すとnil(出品中)に戻す。
func (server *ProductListingServer) UpdateProductListing(ctx context.Context, request *pb.UpdateProductListingRequest) (*pb.UpdateProductListingResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetPrice() < 0 {
		return nil, apperr.InvalidArgument("price must not be negative")
	}

	listing, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product listing", err)
	}

	listing.ProductID = uint(request.GetProductId())
	listing.Price = request.GetPrice()
	listing.Status = request.GetStatus()
	if request.GetListedAt() != nil {
		listing.ListedAt = request.GetListedAt().AsTime()
	}
	if request.GetEndedAt() != nil {
		endedAt := request.GetEndedAt().AsTime()
		listing.EndedAt = &endedAt
	} else {
		listing.EndedAt = nil
	}

	if err := server.repo.Update(listing); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductListingResponse{Listing: toProtoListing(listing)}, nil
}

// DeleteProductListing はIDを指定して出品情報を削除する。
func (server *ProductListingServer) DeleteProductListing(ctx context.Context, request *pb.DeleteProductListingRequest) (*pb.DeleteProductListingResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductListingResponse{}, nil
}

// toProtoListing はDBのmodel.ProductListingをレスポンス用のpb.ProductListingに変換する。
// EndedAt は *time.Time (未終了ならnil) なので、nilなら変換自体をスキップしてprotoのフィールドもnilにする。
func toProtoListing(listing *model.ProductListing) *pb.ProductListing {
	protoListing := &pb.ProductListing{
		Id:        uint32(listing.ID),
		ProductId: uint32(listing.ProductID),
		Price:     listing.Price,
		Status:    listing.Status,
		ListedAt:  timestamppb.New(listing.ListedAt),
		CreatedAt: timestamppb.New(listing.CreatedAt),
		UpdatedAt: timestamppb.New(listing.UpdatedAt),
	}
	if listing.EndedAt != nil {
		protoListing.EndedAt = timestamppb.New(*listing.EndedAt)
	}
	return protoListing
}
