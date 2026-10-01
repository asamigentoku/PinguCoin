package converters

import (
	orcanpb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph/model"
)

// ProductAssetFromPB は gRPC の ProductAsset を、GraphQL の ProductAsset に変換する。nil なら nil を返す。
// 用途(Purpose)は付いているときだけ変換する。
func ProductAssetFromPB(asset *orcanpb.ProductAsset) *model.ProductAsset {
	if asset == nil {
		return nil
	}
	purpose := asset.GetPurpose()
	result := &model.ProductAsset{
		ID: int32(asset.GetId()), ProductID: int32(asset.GetProductId()), PurposeID: int32(asset.GetPurposeId()),
		StorageURL: asset.GetStorageUrl(), OriginalFilename: asset.GetOriginalFilename(), ContentType: asset.GetContentType(),
		FileSize: int32(asset.GetFileSize()), Description: asset.GetDescription(), SortOrder: asset.GetSortOrder(),
		IsPrimary: asset.GetIsPrimary(), Metadata: asset.GetMetadata(),
		CreatedAt: FormatTimestamp(asset.GetCreatedAt().AsTime()), UpdatedAt: FormatTimestamp(asset.GetUpdatedAt().AsTime()),
	}
	if purpose != nil {
		result.Purpose = &model.ProductAssetPurpose{ID: int32(purpose.GetId()), Name: purpose.GetName(), IsPublic: purpose.GetIsPublic()}
	}
	return result
}
