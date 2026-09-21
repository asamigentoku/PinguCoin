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

// ProductCategoryServer は proto の `service ProductCategoryService` を実装する本体。
// pb.UnimplementedProductCategoryServiceServer を埋め込むことで、
// 将来protoにRPCが追加されてもここで実装し忘れたメソッドは
// 「Unimplementedエラーを返すデフォルト実装」で自動的に補われ、コンパイルが壊れない。
type ProductCategoryServer struct {
	pb.UnimplementedProductCategoryServiceServer
	repo *repository.ProductCategoryRepository
}

// NewProductCategoryServer はDBアクセス用のrepositoryを注入してサーバーを作る。
// main.go の pb.RegisterProductCategoryServiceServer(server, ...) でgRPCサーバー本体に登録される。
func NewProductCategoryServer(repo *repository.ProductCategoryRepository) *ProductCategoryServer {
	return &ProductCategoryServer{repo: repo}
}

// 以下5つのメソッドが、クライアントから来た1つのRPC呼び出しに対応する処理。
// 共通の流れ: ①バリデーション → ②repository経由でDBを操作する
// → ③DBのモデル(internal/model)をprotoのメッセージ(pb.XXX)に変換して返す。
// エラーは全て apperr.* (NotFound/InvalidArgument/Internal) で返し、
// 生のDBエラーをクライアントに漏らさない(ログには internal/interceptor.Logging が出す)。

// ListProductCategories はカテゴリ一覧を返す(protoの ListProductCategories RPC の実装)。
func (server *ProductCategoryServer) ListProductCategories(ctx context.Context, request *pb.ListProductCategoriesRequest) (*pb.ListProductCategoriesResponse, error) {
	categories, err := server.repo.FindAll()
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListProductCategoriesResponse{}
	for i := range categories {
		response.Categories = append(response.Categories, toProtoCategory(&categories[i]))
	}
	return response, nil
}

// GetProductCategory はIDを1件指定してカテゴリを取得する。
func (server *ProductCategoryServer) GetProductCategory(ctx context.Context, request *pb.GetProductCategoryRequest) (*pb.GetProductCategoryResponse, error) {
	category, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product category", err)
	}
	return &pb.GetProductCategoryResponse{Category: toProtoCategory(category)}, nil
}

// CreateProductCategory は新規カテゴリを1件作成する。
func (server *ProductCategoryServer) CreateProductCategory(ctx context.Context, request *pb.CreateProductCategoryRequest) (*pb.CreateProductCategoryResponse, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, apperr.InvalidArgument("name is required")
	}

	category := &model.ProductCategory{
		Name: name,
	}
	// parent_id は proto側で `optional` にしているため、
	// 「0」と「未指定」を区別できるようポインタ(*uint32)で来る。
	// request.ParentId != nil で「クライアントが値を送ってきたかどうか」を判定する。
	if request.ParentId != nil {
		parentID := uint(request.GetParentId())
		category.ParentID = &parentID
	}

	if err := server.repo.Create(category); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreateProductCategoryResponse{Category: toProtoCategory(category)}, nil
}

// UpdateProductCategory は既存カテゴリを更新する(いわゆるPUT相当、全項目上書き)。
func (server *ProductCategoryServer) UpdateProductCategory(ctx context.Context, request *pb.UpdateProductCategoryRequest) (*pb.UpdateProductCategoryResponse, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, apperr.InvalidArgument("name is required")
	}

	category, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product category", err)
	}

	category.Name = name
	if request.ParentId != nil {
		parentID := uint(request.GetParentId())
		category.ParentID = &parentID
	} else {
		// 明示的にnilを送られた場合は「親カテゴリなし(ルート)」に更新する。
		category.ParentID = nil
	}

	if err := server.repo.Update(category); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateProductCategoryResponse{Category: toProtoCategory(category)}, nil
}

// DeleteProductCategory はIDを指定してカテゴリを削除する。
func (server *ProductCategoryServer) DeleteProductCategory(ctx context.Context, request *pb.DeleteProductCategoryRequest) (*pb.DeleteProductCategoryResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	// レスポンスにデータが不要なRPCでも、protoの都合上「空のメッセージ」を返す必要がある。
	return &pb.DeleteProductCategoryResponse{}, nil
}

// toProtoCategory はDBのモデル(model.ProductCategory)を
// gRPCでやり取りするためのメッセージ(pb.ProductCategory)に変換するヘルパー。
// DBの型とprotoの型は別物なので、RPCの戻り値を作る際は必ずこの変換を通す。
func toProtoCategory(category *model.ProductCategory) *pb.ProductCategory {
	protoCategory := &pb.ProductCategory{
		Id:        uint32(category.ID),
		Name:      category.Name,
		CreatedAt: timestamppb.New(category.CreatedAt), // time.Time → protobuf標準のTimestamp型へ変換
		UpdatedAt: timestamppb.New(category.UpdatedAt),
	}
	if category.ParentID != nil {
		parentID := uint32(*category.ParentID)
		protoCategory.ParentId = &parentID
	}
	return protoCategory
}
