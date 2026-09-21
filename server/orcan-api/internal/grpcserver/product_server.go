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
func (server *ProductServer) ListProducts(ctx context.Context, request *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	var userID uint
	if request.UserId != nil {
		userID = uint(request.GetUserId())
	}

	products, err := server.repo.FindAll(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListProductsResponse{}
	for i := range products {
		response.Products = append(response.Products, toProtoProduct(&products[i]))
	}
	return response, nil
}

func (server *ProductServer) GetProduct(ctx context.Context, request *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	product, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product", err)
	}
	return &pb.GetProductResponse{Product: toProtoProduct(product)}, nil
}

func (server *ProductServer) CreateProduct(ctx context.Context, request *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	if err := validateProductInput(request.GetName(), request.GetUserId(), request.GetCategoryId()); err != nil {
		return nil, err
	}

	product := &model.Product{
		UserID:      uint(request.GetUserId()),
		CategoryID:  uint(request.GetCategoryId()),
		Name:        strings.TrimSpace(request.GetName()),
		Description: request.GetDescription(),
		ImageURL:    request.GetImageUrl(),
		Price:       request.GetPrice(),
		Status:      request.GetStatus(),
	}

	if err := server.repo.Create(product); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductResponse{Product: toProtoProduct(product)}, nil
}

func (server *ProductServer) UpdateProduct(ctx context.Context, request *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	if err := validateProductInput(request.GetName(), request.GetUserId(), request.GetCategoryId()); err != nil {
		return nil, err
	}

	product, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product", err)
	}

	product.UserID = uint(request.GetUserId())
	product.CategoryID = uint(request.GetCategoryId())
	product.Name = strings.TrimSpace(request.GetName())
	product.Description = request.GetDescription()
	product.ImageURL = request.GetImageUrl()
	product.Price = request.GetPrice()
	product.Status = request.GetStatus()

	if err := server.repo.Update(product); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductResponse{Product: toProtoProduct(product)}, nil
}

func (server *ProductServer) DeleteProduct(ctx context.Context, request *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
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
func toProtoProduct(product *model.Product) *pb.Product {
	return &pb.Product{
		Id:          uint32(product.ID),
		UserId:      uint32(product.UserID),
		CategoryId:  uint32(product.CategoryID),
		Name:        product.Name,
		Description: product.Description,
		ImageUrl:    product.ImageURL,
		Price:       product.Price,
		Status:      product.Status,
		CreatedAt:   timestamppb.New(product.CreatedAt),
		UpdatedAt:   timestamppb.New(product.UpdatedAt),
	}
}
