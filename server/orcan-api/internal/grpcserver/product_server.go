package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/storage"
)

// ProductServer は proto の `service ProductService` の実装。
// パターンはproduct_category_server.goと同じ:
// バリデーション → repository経由でDB操作 → model→pbに変換して返す。
type ProductServer struct {
	pb.UnimplementedProductServiceServer
	repo       *repository.ProductRepository
	detailRepo *repository.ProductDetailRepository
	storage    *storage.BlobStorage
}

func NewProductServer(repo *repository.ProductRepository, detailRepo *repository.ProductDetailRepository, blobStorage *storage.BlobStorage) *ProductServer {
	return &ProductServer{repo: repo, detailRepo: detailRepo, storage: blobStorage}
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

	// 商品画像・ファイルを置く共有コンテナが無ければここで作成する(既にあれば何もしない冪等な呼び出し)。
	if err := server.storage.EnsureContainers(ctx); err != nil {
		return nil, apperr.Internal(err)
	}

	product := &model.Product{
		UserID:      uint(request.GetUserId()),
		CategoryID:  uint(request.GetCategoryId()),
		Name:        strings.TrimSpace(request.GetName()),
		Description: request.GetDescription(),
		ImageURL:    request.GetImageUrl(),
		FileURL:     request.GetFileUrl(),
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
	product.FileURL = request.GetFileUrl()
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

// productOwnedBy は商品を取得し、user_idが所有者と一致するか検証する共通処理。
func (server *ProductServer) productOwnedBy(userID, productID uint32) (*model.Product, error) {
	product, err := server.repo.FindByID(uint(productID))
	if err != nil {
		return nil, mapFindError("product", err)
	}
	if product.UserID != uint(userID) {
		return nil, apperr.PermissionDenied("user does not own this product")
	}
	return product, nil
}

// GetProductImageUploadURL は商品画像(公開コンテナ)をアップロードするための署名付きURL(SAS)を
// 発行する。商品の所有者(user_id)本人からの呼び出しのみ許可する。
func (server *ProductServer) GetProductImageUploadURL(ctx context.Context, request *pb.GetProductImageUploadURLRequest) (*pb.GetProductImageUploadURLResponse, error) {
	if _, err := server.productOwnedBy(request.GetUserId(), request.GetProductId()); err != nil {
		return nil, err
	}

	uploadURL, err := server.storage.IssueImageUploadURL(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.GetProductImageUploadURLResponse{
		BlobEndpoint:    uploadURL.BlobEndpoint,
		Container:       uploadURL.Container,
		MainImagePrefix: server.storage.MainImagePrefix(request.GetUserId(), request.GetProductId()),
		SubImagesPrefix: server.storage.SubImagesPrefix(request.GetUserId(), request.GetProductId()),
		SasToken:        uploadURL.SASToken,
		ExpiresAt:       timestamppb.New(uploadURL.ExpiresAt),
	}, nil
}

// ConfirmProductImageUpload はアップロード完了後に呼ばれる。main_image/配下ならimage_urlとして、
// sub_images/配下ならproduct_detailの1件として保存する。商品の所有者(user_id)本人からの
// 呼び出しのみ許可する。
func (server *ProductServer) ConfirmProductImageUpload(ctx context.Context, request *pb.ConfirmProductImageUploadRequest) (*pb.ConfirmProductImageUploadResponse, error) {
	fileURL := strings.TrimSpace(request.GetFileUrl())
	if fileURL == "" {
		return nil, apperr.InvalidArgument("file_url is required")
	}

	product, err := server.productOwnedBy(request.GetUserId(), request.GetProductId())
	if err != nil {
		return nil, err
	}

	blobName, err := server.storage.ImageBlobNameFromURL(fileURL, request.GetUserId(), request.GetProductId())
	if err != nil {
		return nil, apperr.InvalidArgument("file_url is not under the product's image upload path")
	}
	// SASのクエリ(署名は有効期限付き)を含めたまま永続化すると期限切れ後にURLが無効になる。
	// 画像は公開コンテナなのでSASを除いた素のURLがそのまま恒久的な公開URLとして使える。
	cleanURL := stripQuery(fileURL)

	switch server.storage.ClassifyImageBlobName(blobName, request.GetUserId(), request.GetProductId()) {
	case storage.ImageKindMain:
		product.ImageURL = cleanURL
		if err := server.repo.Update(product); err != nil {
			return nil, apperr.Internal(err)
		}
		return &pb.ConfirmProductImageUploadResponse{Product: toProtoProduct(product)}, nil

	case storage.ImageKindSub:
		detail := &model.ProductDetail{
			ProductID:   product.ID,
			ImageURL:    cleanURL,
			Description: request.GetDescription(),
			SortOrder:   int(request.GetSortOrder()),
		}
		if err := server.detailRepo.Create(detail); err != nil {
			return nil, apperr.Internal(err)
		}
		return &pb.ConfirmProductImageUploadResponse{
			Product:           toProtoProduct(product),
			DetailId:          uint32(detail.ID),
			DetailImageUrl:    detail.ImageURL,
			DetailDescription: detail.Description,
			DetailSortOrder:   int32(detail.SortOrder),
		}, nil

	default:
		return nil, apperr.InvalidArgument("file_url must be under main_image/ or sub_images/")
	}
}

// DeleteProductImageUpload はアップロード済みの画像ファイルを削除する。main_image/配下で、
// 現在のimage_urlと一致する場合はimage_urlも空にする。sub_images/配下の場合は対応する
// product_detailのレコードも削除する。商品の所有者(user_id)本人からの呼び出しのみ許可する。
func (server *ProductServer) DeleteProductImageUpload(ctx context.Context, request *pb.DeleteProductImageUploadRequest) (*pb.DeleteProductImageUploadResponse, error) {
	fileURL := strings.TrimSpace(request.GetFileUrl())
	if fileURL == "" {
		return nil, apperr.InvalidArgument("file_url is required")
	}

	product, err := server.productOwnedBy(request.GetUserId(), request.GetProductId())
	if err != nil {
		return nil, err
	}

	blobName, err := server.storage.ImageBlobNameFromURL(fileURL, request.GetUserId(), request.GetProductId())
	if err != nil {
		if errors.Is(err, storage.ErrBlobNotInProductPath) {
			return nil, apperr.InvalidArgument("file_url is not under the product's image upload path")
		}
		return nil, apperr.Internal(err)
	}

	if err := server.storage.DeleteImageBlob(ctx, blobName); err != nil {
		return nil, apperr.Internal(err)
	}

	cleanURL := stripQuery(fileURL)
	switch server.storage.ClassifyImageBlobName(blobName, request.GetUserId(), request.GetProductId()) {
	case storage.ImageKindMain:
		if product.ImageURL == cleanURL {
			product.ImageURL = ""
			if err := server.repo.Update(product); err != nil {
				return nil, apperr.Internal(err)
			}
		}
	case storage.ImageKindSub:
		if err := server.detailRepo.DeleteByImageURL(product.ID, cleanURL); err != nil {
			return nil, apperr.Internal(err)
		}
	}

	return &pb.DeleteProductImageUploadResponse{Product: toProtoProduct(product)}, nil
}

// GetProductFileUploadURL は商品ファイル(非公開コンテナ)をアップロードするための署名付きURL(SAS)を
// 発行する。商品の所有者(user_id)本人からの呼び出しのみ許可する。
func (server *ProductServer) GetProductFileUploadURL(ctx context.Context, request *pb.GetProductFileUploadURLRequest) (*pb.GetProductFileUploadURLResponse, error) {
	if _, err := server.productOwnedBy(request.GetUserId(), request.GetProductId()); err != nil {
		return nil, err
	}

	uploadURL, err := server.storage.IssueFileUploadURL(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.GetProductFileUploadURLResponse{
		BlobEndpoint: uploadURL.BlobEndpoint,
		Container:    uploadURL.Container,
		PathPrefix:   server.storage.ProductFilePrefix(request.GetUserId(), request.GetProductId()),
		SasToken:     uploadURL.SASToken,
		ExpiresAt:    timestamppb.New(uploadURL.ExpiresAt),
	}, nil
}

// ConfirmProductFileUpload はアップロード完了後に呼ばれ、実際に置かれたファイルのURLを
// 商品のfile_urlとして保存する。商品の所有者(user_id)本人からの呼び出しのみ許可する。
func (server *ProductServer) ConfirmProductFileUpload(ctx context.Context, request *pb.ConfirmProductFileUploadRequest) (*pb.ConfirmProductFileUploadResponse, error) {
	fileURL := strings.TrimSpace(request.GetFileUrl())
	if fileURL == "" {
		return nil, apperr.InvalidArgument("file_url is required")
	}

	product, err := server.productOwnedBy(request.GetUserId(), request.GetProductId())
	if err != nil {
		return nil, err
	}

	if _, err := server.storage.FileBlobNameFromURL(fileURL, request.GetUserId(), request.GetProductId()); err != nil {
		return nil, apperr.InvalidArgument("file_url is not under the product's file upload path")
	}

	product.FileURL = stripQuery(fileURL)
	if err := server.repo.Update(product); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.ConfirmProductFileUploadResponse{Product: toProtoProduct(product)}, nil
}

// DeleteProductFileUpload はアップロード済みの商品ファイルを削除する。削除したファイルが
// 現在のfile_urlと一致する場合は、file_urlも空にする。商品の所有者(user_id)本人からの
// 呼び出しのみ許可する。
func (server *ProductServer) DeleteProductFileUpload(ctx context.Context, request *pb.DeleteProductFileUploadRequest) (*pb.DeleteProductFileUploadResponse, error) {
	fileURL := strings.TrimSpace(request.GetFileUrl())
	if fileURL == "" {
		return nil, apperr.InvalidArgument("file_url is required")
	}

	product, err := server.productOwnedBy(request.GetUserId(), request.GetProductId())
	if err != nil {
		return nil, err
	}

	blobName, err := server.storage.FileBlobNameFromURL(fileURL, request.GetUserId(), request.GetProductId())
	if err != nil {
		if errors.Is(err, storage.ErrBlobNotInProductPath) {
			return nil, apperr.InvalidArgument("file_url is not under the product's file upload path")
		}
		return nil, apperr.Internal(err)
	}

	if err := server.storage.DeleteFileBlob(ctx, blobName); err != nil {
		return nil, apperr.Internal(err)
	}

	if product.FileURL == stripQuery(fileURL) {
		product.FileURL = ""
		if err := server.repo.Update(product); err != nil {
			return nil, apperr.Internal(err)
		}
	}

	return &pb.DeleteProductFileUploadResponse{Product: toProtoProduct(product)}, nil
}

// GetProductDownloadURL は商品ファイル(file_url)をダウンロードするための署名付きURLを発行する。
// orcan-apiは購入状況を持たないため、呼び出し元(pingu-api)が事前に権限を検証していることを
// 前提とし、ここでは所有者チェック等は行わない。
func (server *ProductServer) GetProductDownloadURL(ctx context.Context, request *pb.GetProductDownloadURLRequest) (*pb.GetProductDownloadURLResponse, error) {
	product, err := server.repo.FindByID(uint(request.GetProductId()))
	if err != nil {
		return nil, mapFindError("product", err)
	}
	if product.FileURL == "" {
		return nil, apperr.InvalidArgument("product has no file_url")
	}

	blobName, err := server.storage.FileBlobNameFromURL(product.FileURL, uint32(product.UserID), uint32(product.ID))
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("stored file_url is invalid: %w", err))
	}

	downloadURL, expiresAt, err := server.storage.IssueDownloadURL(ctx, blobName)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.GetProductDownloadURLResponse{
		DownloadUrl: downloadURL,
		ExpiresAt:   timestamppb.New(expiresAt),
	}, nil
}

// stripQuery はURLからクエリ文字列(SASトークン等)を取り除く。パースに失敗した場合は
// (バリデーション済みの呼び出し元では通常起きないが)元の文字列をそのまま返す。
func stripQuery(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String()
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
		FileUrl:     product.FileURL,
		Price:       product.Price,
		Status:      product.Status,
		CreatedAt:   timestamppb.New(product.CreatedAt),
		UpdatedAt:   timestamppb.New(product.UpdatedAt),
	}
}
