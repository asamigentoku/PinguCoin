package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/storage"
	pb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
)

type ProductAssetServer struct {
	pb.UnimplementedProductAssetServiceServer
	assets   *repository.ProductAssetRepository
	products *repository.ProductRepository
	storage  *storage.BlobStorage
}

func NewProductAssetServer(assets *repository.ProductAssetRepository, products *repository.ProductRepository, blobStorage *storage.BlobStorage) *ProductAssetServer {
	return &ProductAssetServer{assets: assets, products: products, storage: blobStorage}
}

func (server *ProductAssetServer) ListProductAssets(_ context.Context, request *pb.ListProductAssetsRequest) (*pb.ListProductAssetsResponse, error) {
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	var purposeID uint16
	if request.PurposeId != nil {
		purposeID = uint16(request.GetPurposeId())
	}
	assets, err := server.assets.FindAll(uint(request.GetProductId()), purposeID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	response := &pb.ListProductAssetsResponse{Assets: make([]*pb.ProductAsset, 0, len(assets))}
	for i := range assets {
		response.Assets = append(response.Assets, toProtoProductAsset(&assets[i]))
	}
	return response, nil
}

func (server *ProductAssetServer) GetProductAsset(_ context.Context, request *pb.GetProductAssetRequest) (*pb.GetProductAssetResponse, error) {
	asset, err := server.assets.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("product asset", err)
	}
	return &pb.GetProductAssetResponse{Asset: toProtoProductAsset(asset)}, nil
}

func (server *ProductAssetServer) GetProductAssetUploadURL(ctx context.Context, request *pb.GetProductAssetUploadURLRequest) (*pb.GetProductAssetUploadURLResponse, error) {
	if _, err := server.productOwnedBy(request.GetUserId(), request.GetProductId()); err != nil {
		return nil, err
	}
	purpose, err := server.findPurpose(request.GetPurposeId())
	if err != nil {
		return nil, err
	}
	target, err := server.storage.IssueAssetUploadURL(ctx, purpose.IsPublic)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.GetProductAssetUploadURLResponse{
		BlobEndpoint: target.BlobEndpoint,
		Container:    target.Container,
		PathPrefix:   server.storage.ProductAssetPrefix(request.GetUserId(), request.GetProductId(), purpose.ID),
		SasToken:     target.SASToken,
		ExpiresAt:    timestamppb.New(target.ExpiresAt),
	}, nil
}

func (server *ProductAssetServer) ConfirmProductAssetUpload(_ context.Context, request *pb.ConfirmProductAssetUploadRequest) (*pb.ConfirmProductAssetUploadResponse, error) {
	fileURL := strings.TrimSpace(request.GetFileUrl())
	if fileURL == "" {
		return nil, apperr.InvalidArgument("file_url is required")
	}
	if request.GetFileSize() < 0 {
		return nil, apperr.InvalidArgument("file_size must not be negative")
	}
	if _, err := server.productOwnedBy(request.GetUserId(), request.GetProductId()); err != nil {
		return nil, err
	}
	purpose, err := server.findPurpose(request.GetPurposeId())
	if err != nil {
		return nil, err
	}
	if _, err := server.storage.AssetBlobNameFromURL(fileURL, request.GetUserId(), request.GetProductId(), purpose.ID, purpose.IsPublic); err != nil {
		return nil, apperr.InvalidArgument("file_url is not under the product asset upload path")
	}
	metadata := strings.TrimSpace(request.GetMetadata())
	if metadata == "" {
		metadata = "{}"
	}
	if !json.Valid([]byte(metadata)) {
		return nil, apperr.InvalidArgument("metadata must be valid JSON")
	}
	asset := &model.ProductAsset{
		ProductID:        uint(request.GetProductId()),
		PurposeID:        purpose.ID,
		Purpose:          *purpose,
		StorageURL:       stripQuery(fileURL),
		OriginalFilename: strings.TrimSpace(request.GetOriginalFilename()),
		ContentType:      strings.TrimSpace(request.GetContentType()),
		FileSize:         request.GetFileSize(),
		Description:      request.GetDescription(),
		SortOrder:        int(request.GetSortOrder()),
		IsPrimary:        request.GetIsPrimary(),
		Metadata:         metadata,
	}
	if err := server.assets.Create(asset); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.ConfirmProductAssetUploadResponse{Asset: toProtoProductAsset(asset)}, nil
}

func (server *ProductAssetServer) DeleteProductAsset(ctx context.Context, request *pb.DeleteProductAssetRequest) (*pb.DeleteProductAssetResponse, error) {
	asset, err := server.assets.FindByID(uint(request.GetAssetId()))
	if err != nil {
		return nil, mapFindError("product asset", err)
	}
	if _, err := server.productOwnedBy(request.GetUserId(), uint32(asset.ProductID)); err != nil {
		return nil, err
	}
	blobName, err := server.storage.AssetBlobNameFromURL(asset.StorageURL, request.GetUserId(), uint32(asset.ProductID), asset.PurposeID, asset.Purpose.IsPublic)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if err := server.storage.DeleteAssetBlob(ctx, blobName, asset.Purpose.IsPublic); err != nil {
		return nil, apperr.Internal(err)
	}
	if err := server.assets.Delete(asset.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteProductAssetResponse{}, nil
}

func (server *ProductAssetServer) GetProductAssetDownloadURL(ctx context.Context, request *pb.GetProductAssetDownloadURLRequest) (*pb.GetProductAssetDownloadURLResponse, error) {
	asset, err := server.assets.FindByID(uint(request.GetAssetId()))
	if err != nil {
		return nil, mapFindError("product asset", err)
	}
	if asset.Purpose.IsPublic {
		return nil, apperr.InvalidArgument("public assets can be read from storage_url")
	}
	product, err := server.products.FindByID(asset.ProductID)
	if err != nil {
		return nil, mapFindError("product", err)
	}
	blobName, err := server.storage.AssetBlobNameFromURL(asset.StorageURL, uint32(product.UserID), uint32(product.ID), asset.PurposeID, false)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	downloadURL, expiresAt, err := server.storage.IssueDownloadURL(ctx, blobName)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.GetProductAssetDownloadURLResponse{DownloadUrl: downloadURL, ExpiresAt: timestamppb.New(expiresAt)}, nil
}

func (server *ProductAssetServer) productOwnedBy(userID, productID uint32) (*model.Product, error) {
	product, err := server.products.FindByID(uint(productID))
	if err != nil {
		return nil, mapFindError("product", err)
	}
	if product.UserID != uint(userID) {
		return nil, apperr.PermissionDenied("product is not owned by user")
	}
	return product, nil
}

func (server *ProductAssetServer) findPurpose(id uint32) (*model.ProductAssetPurpose, error) {
	if id == 0 || id > uint32(^uint16(0)) {
		return nil, apperr.InvalidArgument("purpose_id is invalid")
	}
	purpose, err := server.assets.FindPurposeByID(uint16(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.InvalidArgument("purpose_id is not registered")
		}
		return nil, apperr.Internal(err)
	}
	return purpose, nil
}

func toProtoProductAsset(asset *model.ProductAsset) *pb.ProductAsset {
	return &pb.ProductAsset{
		Id:               uint32(asset.ID),
		ProductId:        uint32(asset.ProductID),
		PurposeId:        uint32(asset.PurposeID),
		Purpose:          &pb.ProductAssetPurpose{Id: uint32(asset.Purpose.ID), Name: asset.Purpose.Name, IsPublic: asset.Purpose.IsPublic},
		StorageUrl:       asset.StorageURL,
		OriginalFilename: asset.OriginalFilename,
		ContentType:      asset.ContentType,
		FileSize:         asset.FileSize,
		Description:      asset.Description,
		SortOrder:        int32(asset.SortOrder),
		IsPrimary:        asset.IsPrimary,
		Metadata:         asset.Metadata,
		CreatedAt:        timestamppb.New(asset.CreatedAt),
		UpdatedAt:        timestamppb.New(asset.UpdatedAt),
	}
}
