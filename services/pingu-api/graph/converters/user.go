package converters

import (
	orcanpb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph/model"
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
