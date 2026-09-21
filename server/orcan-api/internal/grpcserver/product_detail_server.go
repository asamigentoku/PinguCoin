package grpcserver

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

// ProductDetailServer は proto の `service ProductDetailService` の実装。
// 1商品(product_id)につき複数件の画像・詳細(ProductDetail)を持てる想定。
type ProductDetailServer struct {
	pb.UnimplementedProductDetailServiceServer
	repo *repository.ProductDetailRepository
}

func NewProductDetailServer(repo *repository.ProductDetailRepository) *ProductDetailServer {
	return &ProductDetailServer{repo: repo}
}

// ListProductDetails は詳細一覧を返す。product_id を指定するとその商品の詳細だけに絞り込む。
func (server *ProductDetailServer) ListProductDetails(ctx context.Context, request *pb.ListProductDetailsRequest) (*pb.ListProductDetailsResponse, error) {
	var productID uint
	if request.ProductId != nil {
		productID = uint(request.GetProductId())
	}

	details, err := server.repo.FindAll(productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListProductDetailsResponse{}
	for i := range details {
		response.Details = append(response.Details, toProtoDetail(&details[i]))
	}
	return response, nil
}

func (server *ProductDetailServer) GetProductDetail(ctx context.Context, request *pb.GetProductDetailRequest) (*pb.GetProductDetailResponse, error) {
	detail, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product detail", err)
	}
	return &pb.GetProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (server *ProductDetailServer) CreateProductDetail(ctx context.Context, request *pb.CreateProductDetailRequest) (*pb.CreateProductDetailResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}

	detail := &model.ProductDetail{
		ProductID:   uint(request.GetProductId()),
		ImageURL:    request.GetImageUrl(),
		Description: request.GetDescription(),
		SortOrder:   int(request.GetSortOrder()),
	}

	if err := server.repo.Create(detail); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (server *ProductDetailServer) UpdateProductDetail(ctx context.Context, request *pb.UpdateProductDetailRequest) (*pb.UpdateProductDetailResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}

	detail, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product detail", err)
	}

	detail.ProductID = uint(request.GetProductId())
	detail.ImageURL = request.GetImageUrl()
	detail.Description = request.GetDescription()
	detail.SortOrder = int(request.GetSortOrder())

	if err := server.repo.Update(detail); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (server *ProductDetailServer) DeleteProductDetail(ctx context.Context, request *pb.DeleteProductDetailRequest) (*pb.DeleteProductDetailResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductDetailResponse{}, nil
}

// toProtoDetail はDBのmodel.ProductDetailをレスポンス用のpb.ProductDetailに変換する。
func toProtoDetail(detail *model.ProductDetail) *pb.ProductDetail {
	return &pb.ProductDetail{
		Id:          uint32(detail.ID),
		ProductId:   uint32(detail.ProductID),
		ImageUrl:    detail.ImageURL,
		Description: detail.Description,
		SortOrder:   int32(detail.SortOrder),
		CreatedAt:   timestamppb.New(detail.CreatedAt),
		UpdatedAt:   timestamppb.New(detail.UpdatedAt),
	}
}
