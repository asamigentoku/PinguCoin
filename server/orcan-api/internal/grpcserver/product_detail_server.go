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
func (s *ProductDetailServer) ListProductDetails(ctx context.Context, req *pb.ListProductDetailsRequest) (*pb.ListProductDetailsResponse, error) {
	var productID uint
	if req.ProductId != nil {
		productID = uint(req.GetProductId())
	}

	details, err := s.repo.FindAll(productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListProductDetailsResponse{}
	for i := range details {
		resp.Details = append(resp.Details, toProtoDetail(&details[i]))
	}
	return resp, nil
}

func (s *ProductDetailServer) GetProductDetail(ctx context.Context, req *pb.GetProductDetailRequest) (*pb.GetProductDetailResponse, error) {
	detail, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product detail", err)
	}
	return &pb.GetProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (s *ProductDetailServer) CreateProductDetail(ctx context.Context, req *pb.CreateProductDetailRequest) (*pb.CreateProductDetailResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}

	detail := &model.ProductDetail{
		ProductID:   uint(req.GetProductId()),
		ImageURL:    req.GetImageUrl(),
		Description: req.GetDescription(),
		SortOrder:   int(req.GetSortOrder()),
	}

	if err := s.repo.Create(detail); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (s *ProductDetailServer) UpdateProductDetail(ctx context.Context, req *pb.UpdateProductDetailRequest) (*pb.UpdateProductDetailResponse, error) {
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}

	detail, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product detail", err)
	}

	detail.ProductID = uint(req.GetProductId())
	detail.ImageURL = req.GetImageUrl()
	detail.Description = req.GetDescription()
	detail.SortOrder = int(req.GetSortOrder())

	if err := s.repo.Update(detail); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductDetailResponse{Detail: toProtoDetail(detail)}, nil
}

func (s *ProductDetailServer) DeleteProductDetail(ctx context.Context, req *pb.DeleteProductDetailRequest) (*pb.DeleteProductDetailResponse, error) {
	if err := s.repo.Delete(uint(req.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductDetailResponse{}, nil
}

// toProtoDetail はDBのmodel.ProductDetailをレスポンス用のpb.ProductDetailに変換する。
func toProtoDetail(d *model.ProductDetail) *pb.ProductDetail {
	return &pb.ProductDetail{
		Id:          uint32(d.ID),
		ProductId:   uint32(d.ProductID),
		ImageUrl:    d.ImageURL,
		Description: d.Description,
		SortOrder:   int32(d.SortOrder),
		CreatedAt:   timestamppb.New(d.CreatedAt),
		UpdatedAt:   timestamppb.New(d.UpdatedAt),
	}
}
