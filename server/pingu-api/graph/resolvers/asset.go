package resolvers

import (
	"context"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/converters"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/reqcontext"
)

func (r *mutationResolver) GetProductAssetUploadURL(ctx context.Context, productID int32, purposeID int32) (*model.ProductAssetUploadTarget, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Asset.GetProductAssetUploadURL(ctx, &orcanpb.GetProductAssetUploadURLRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID), PurposeId: uint32(purposeID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return &model.ProductAssetUploadTarget{
		BlobEndpoint: response.GetBlobEndpoint(), Container: response.GetContainer(), PathPrefix: response.GetPathPrefix(),
		SasToken: response.GetSasToken(), ExpiresAt: converters.FormatTimestamp(response.GetExpiresAt().AsTime()),
	}, nil
}

func (r *mutationResolver) ConfirmProductAssetUpload(ctx context.Context, productID int32, purposeID int32, fileURL string, input *model.ConfirmProductAssetInput) (*model.ProductAsset, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	request := &orcanpb.ConfirmProductAssetUploadRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID), PurposeId: uint32(purposeID), FileUrl: fileURL}
	if input != nil {
		request.OriginalFilename = converters.StringOrEmpty(input.OriginalFilename)
		request.ContentType = converters.StringOrEmpty(input.ContentType)
		if input.FileSize != nil {
			request.FileSize = int64(*input.FileSize)
		}
		request.Description = converters.StringOrEmpty(input.Description)
		if input.SortOrder != nil {
			request.SortOrder = *input.SortOrder
		}
		if input.IsPrimary != nil {
			request.IsPrimary = *input.IsPrimary
		}
		request.Metadata = converters.StringOrEmpty(input.Metadata)
	}
	response, err := r.Orcan.Asset.ConfirmProductAssetUpload(ctx, request)
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductAssetFromPB(response.GetAsset()), nil
}

func (r *mutationResolver) DeleteProductAsset(ctx context.Context, assetID int32) (bool, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return false, apperr.Unauthenticated("login is required")
	}
	if _, err := r.Orcan.Asset.DeleteProductAsset(ctx, &orcanpb.DeleteProductAssetRequest{UserId: uint32(claims.UserID), AssetId: uint32(assetID)}); err != nil {
		return false, apperr.FromGRPC(err)
	}
	return true, nil
}

func (r *mutationResolver) GetProductAssetDownloadURL(ctx context.Context, assetID int32) (*model.ProductDownloadTarget, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	assetResponse, err := r.Orcan.Asset.GetProductAsset(ctx, &orcanpb.GetProductAssetRequest{Id: uint32(assetID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	asset := assetResponse.GetAsset()
	if asset.GetPurpose().GetIsPublic() {
		return nil, apperr.InvalidArgument("public asset uses storageUrl")
	}
	productResponse, err := r.Orcan.Product.GetProduct(ctx, &orcanpb.GetProductRequest{Id: asset.GetProductId()})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	if productResponse.GetProduct().GetUserId() != uint32(claims.UserID) {
		purchased, err := r.Orders.HasPaidOrder(claims.UserID, uint(asset.GetProductId()))
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if !purchased {
			return nil, apperr.Unauthenticated("product not purchased")
		}
	}
	response, err := r.Orcan.Asset.GetProductAssetDownloadURL(ctx, &orcanpb.GetProductAssetDownloadURLRequest{AssetId: uint32(assetID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return &model.ProductDownloadTarget{DownloadURL: response.GetDownloadUrl(), ExpiresAt: converters.FormatTimestamp(response.GetExpiresAt().AsTime())}, nil
}

func (r *queryResolver) ProductAssets(ctx context.Context, productID int32, purposeID *int32) ([]*model.ProductAsset, error) {
	request := &orcanpb.ListProductAssetsRequest{ProductId: uint32(productID)}
	if purposeID != nil {
		value := uint32(*purposeID)
		request.PurposeId = &value
	}
	response, err := r.Orcan.Asset.ListProductAssets(ctx, request)
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	canSeePrivate, err := r.canSeePrivateAssets(ctx, uint32(productID))
	if err != nil {
		return nil, err
	}
	assets := make([]*model.ProductAsset, 0, len(response.GetAssets()))
	for _, asset := range response.GetAssets() {
		if !asset.GetPurpose().GetIsPublic() && !canSeePrivate {
			continue
		}
		assets = append(assets, converters.ProductAssetFromPB(asset))
	}
	return assets, nil
}

func (r *queryResolver) canSeePrivateAssets(ctx context.Context, productID uint32) (bool, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return false, nil
	}
	productResponse, err := r.Orcan.Product.GetProduct(ctx, &orcanpb.GetProductRequest{Id: productID})
	if err != nil {
		return false, apperr.FromGRPC(err)
	}
	if productResponse.GetProduct().GetUserId() == uint32(claims.UserID) {
		return true, nil
	}
	purchased, err := r.Orders.HasPaidOrder(claims.UserID, uint(productID))
	if err != nil {
		return false, apperr.Internal(err)
	}
	return purchased, nil
}
