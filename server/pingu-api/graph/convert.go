package graph

import (
	"time"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

// このファイルはgqlgenの再生成対象外(schema.resolvers.goではない)。
// orcan-apiのgRPCメッセージ(pb.*)とGraphQLモデル(model.*)の変換を集約する。

func productFromPB(p *orcanpb.Product) *model.Product {
	if p == nil {
		return nil
	}
	return &model.Product{
		ID:          int32(p.GetId()),
		UserID:      int32(p.GetUserId()),
		CategoryID:  int32(p.GetCategoryId()),
		Name:        p.GetName(),
		Description: p.GetDescription(),
		ImageURL:    p.GetImageUrl(),
		Price:       int32(p.GetPrice()),
		Status:      p.GetStatus(),
		CreatedAt:   formatTimestamp(p.GetCreatedAt().AsTime()),
		UpdatedAt:   formatTimestamp(p.GetUpdatedAt().AsTime()),
	}
}

func userFromPB(u *orcanpb.User) *model.User {
	if u == nil {
		return nil
	}
	return &model.User{
		ID:        int32(u.GetId()),
		Email:     u.GetEmail(),
		Name:      u.GetName(),
		CreatedAt: formatTimestamp(u.GetCreatedAt().AsTime()),
		UpdatedAt: formatTimestamp(u.GetUpdatedAt().AsTime()),
	}
}

func formatTimestamp(t time.Time) string {
	return t.Format(time.RFC3339)
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
