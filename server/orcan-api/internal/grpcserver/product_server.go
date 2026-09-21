package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

// ProductServer は proto の `service ProductService` の実装。
// パターンはproduct_category_server.goと同じ:
// バリデーション → repository経由でDB操作 → model→pbに変換して返す。
type ProductServer struct {
	pb.UnimplementedProductServiceServer
	repo *repository.ProductRepository
}

func NewProductServer(repo *repository.ProductRepository) *ProductServer {
	return &ProductServer{repo: repo}
}

// ListProducts は商品一覧を返す。user_id はproto側でoptionalなので、
// 指定なし(nil)なら全件、指定ありなら出品者で絞り込む。
func (s *ProductServer) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	var userID uint
	if req.UserId != nil {
		userID = uint(req.GetUserId())
	}

	products, err := s.repo.FindAll(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListProductsResponse{}
	for i := range products {
		resp.Products = append(resp.Products, toProtoProduct(&products[i]))
	}
	return resp, nil
}

func (s *ProductServer) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	product, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product", err)
	}
	return &pb.GetProductResponse{Product: toProtoProduct(product)}, nil
}

func (s *ProductServer) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	if err := validateProductInput(req.GetName(), req.GetUserId(), req.GetCategoryId()); err != nil {
		return nil, err
	}

	product := &model.Product{
		UserID:      uint(req.GetUserId()),
		CategoryID:  uint(req.GetCategoryId()),
		Name:        strings.TrimSpace(req.GetName()),
		Description: req.GetDescription(),
		ImageURL:    req.GetImageUrl(),
		Price:       req.GetPrice(),
		Status:      req.GetStatus(),
	}

	if err := s.repo.Create(product); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductResponse{Product: toProtoProduct(product)}, nil
}

func (s *ProductServer) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	if err := validateProductInput(req.GetName(), req.GetUserId(), req.GetCategoryId()); err != nil {
		return nil, err
	}

	product, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("product", err)
	}

	product.UserID = uint(req.GetUserId())
	product.CategoryID = uint(req.GetCategoryId())
	product.Name = strings.TrimSpace(req.GetName())
	product.Description = req.GetDescription()
	product.ImageURL = req.GetImageUrl()
	product.Price = req.GetPrice()
	product.Status = req.GetStatus()

	if err := s.repo.Update(product); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductResponse{Product: toProtoProduct(product)}, nil
}

func (s *ProductServer) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	if err := s.repo.Delete(uint(req.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductResponse{}, nil
}

// validateProductInput はCreate/Updateで共通の入力チェック。
// name, user_id(出品者), category_id は商品として意味を持つために必須とする。
func validateProductInput(name string, userID, categoryID uint32) error {
	if strings.TrimSpace(name) == "" {
		return apperr.InvalidArgument("name is required")
	}
	if userID == 0 {
		return apperr.InvalidArgument("user_id is required")
	}
	if categoryID == 0 {
		return apperr.InvalidArgument("category_id is required")
	}
	return nil
}

// toProtoProduct はDBのmodel.Productをレスポンス用のpb.Productに変換する。
func toProtoProduct(p *model.Product) *pb.Product {
	return &pb.Product{
		Id:          uint32(p.ID),
		UserId:      uint32(p.UserID),
		CategoryId:  uint32(p.CategoryID),
		Name:        p.Name,
		Description: p.Description,
		ImageUrl:    p.ImageURL,
		Price:       p.Price,
		Status:      p.Status,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}
