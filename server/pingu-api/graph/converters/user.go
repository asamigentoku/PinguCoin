package converters

import (
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

// UserFromPB は gRPC の User を、GraphQL の User に変換する。nil なら nil を返す。
func UserFromPB(user *orcanpb.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID: int32(user.GetId()), Email: user.GetEmail(), Name: user.GetName(),
		CreatedAt: FormatTimestamp(user.GetCreatedAt().AsTime()), UpdatedAt: FormatTimestamp(user.GetUpdatedAt().AsTime()),
	}
}
