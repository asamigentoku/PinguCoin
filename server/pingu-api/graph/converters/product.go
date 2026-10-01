package converters

import (
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

// ProductFromPB は gRPC の Product を、GraphQL の Product に変換する。nil なら nil を返す。
func ProductFromPB(product *orcanpb.Product) *model.Product {
	if product == nil {
		return nil
	}
	return &model.Product{
		ID: int32(product.GetId()), UserID: int32(product.GetUserId()), CategoryID: int32(product.GetCategoryId()),
		Name: product.GetName(), Description: product.GetDescription(), ImageURL: product.GetImageUrl(),
		FileURL: product.GetFileUrl(), Price: int32(product.GetPrice()), Status: product.GetStatus(),
		CreatedAt: FormatTimestamp(product.GetCreatedAt().AsTime()), UpdatedAt: FormatTimestamp(product.GetUpdatedAt().AsTime()),
		Version: int32(product.GetVersion()),
	}
}
