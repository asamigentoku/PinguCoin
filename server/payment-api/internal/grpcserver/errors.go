package grpcserver

import (
	"errors"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/apperr"
)

// mapFindError は repository の FindByID 等が返すエラーを、
// レコード未検出なら apperr.NotFound に、それ以外は apperr.Internal に変換する。
func mapFindError(resource string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound(resource, err)
	}
	return apperr.Internal(err)
}
