package graph

import (
	"time"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

// このファイルはgqlgenの再生成対象外(schema.resolvers.goではない)。
// orcan-apiのgRPCメッセージ(pb.*)とGraphQLモデル(model.*)の変換を集約する。

func productFromPB(product *orcanpb.Product) *model.Product {
	if product == nil {
		return nil
	}
	return &model.Product{
		ID:          int32(product.GetId()),
		UserID:      int32(product.GetUserId()),
		CategoryID:  int32(product.GetCategoryId()),
		Name:        product.GetName(),
		Description: product.GetDescription(),
		ImageURL:    product.GetImageUrl(),
		FileURL:     product.GetFileUrl(),
		Price:       int32(product.GetPrice()),
		Status:      product.GetStatus(),
		CreatedAt:   formatTimestamp(product.GetCreatedAt().AsTime()),
		UpdatedAt:   formatTimestamp(product.GetUpdatedAt().AsTime()),
		Version:     int32(product.GetVersion()),
	}
}

func productAssetFromPB(asset *orcanpb.ProductAsset) *model.ProductAsset {
	if asset == nil {
		return nil
	}
	purpose := asset.GetPurpose()
	result := &model.ProductAsset{
		ID: int32(asset.GetId()), ProductID: int32(asset.GetProductId()), PurposeID: int32(asset.GetPurposeId()),
		StorageURL: asset.GetStorageUrl(), OriginalFilename: asset.GetOriginalFilename(), ContentType: asset.GetContentType(),
		FileSize: int32(asset.GetFileSize()), Description: asset.GetDescription(), SortOrder: asset.GetSortOrder(),
		IsPrimary: asset.GetIsPrimary(), Metadata: asset.GetMetadata(),
		CreatedAt: formatTimestamp(asset.GetCreatedAt().AsTime()), UpdatedAt: formatTimestamp(asset.GetUpdatedAt().AsTime()),
	}
	if purpose != nil {
		result.Purpose = &model.ProductAssetPurpose{ID: int32(purpose.GetId()), Name: purpose.GetName(), IsPublic: purpose.GetIsPublic()}
	}
	return result
}

func userFromPB(user *orcanpb.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:        int32(user.GetId()),
		Email:     user.GetEmail(),
		Name:      user.GetName(),
		CreatedAt: formatTimestamp(user.GetCreatedAt().AsTime()),
		UpdatedAt: formatTimestamp(user.GetUpdatedAt().AsTime()),
	}
}

func formatTimestamp(timestamp time.Time) string {
	return timestamp.Format(time.RFC3339)
}

func strOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
