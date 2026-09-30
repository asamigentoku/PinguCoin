package converters

import (
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

func UserFromPB(user *orcanpb.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID: int32(user.GetId()), Email: user.GetEmail(), Name: user.GetName(),
		CreatedAt: FormatTimestamp(user.GetCreatedAt().AsTime()), UpdatedAt: FormatTimestamp(user.GetUpdatedAt().AsTime()),
	}
}
