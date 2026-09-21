package grpcserver

import (
	"errors"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
)

// mapFindError は repository の FindByID 等が返すエラーを、
// レコード未検出なら apperr.NotFound に、それ以外は apperr.Internal に変換する。
// DBの生のエラー(gorm.ErrRecordNotFound以外)はクライアントには返さず、
// ログ用に apperr.Internal の中に保持したまま返す。
func mapFindError(resource string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound(resource, err)
	}
	return apperr.Internal(err)
}
