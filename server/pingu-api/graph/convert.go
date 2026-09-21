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
		Price:       int32(product.GetPrice()),
		Status:      product.GetStatus(),
		CreatedAt:   formatTimestamp(product.GetCreatedAt().AsTime()),
		UpdatedAt:   formatTimestamp(product.GetUpdatedAt().AsTime()),
	}
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
