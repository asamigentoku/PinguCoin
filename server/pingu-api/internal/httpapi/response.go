package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorResponse struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// writeError はエラーをJSONレスポンスに変換する。*apperr.AppError以外(想定外のエラー)は
// 内部情報を漏らさないよう500 internal server errorとして扱う。
func writeError(w http.ResponseWriter, err error) {
	appErr, ok := apperr.As(err)
	if !ok {
		appErr = apperr.Internal(err)
	}
	// 元エラー(DBエラー等)はクライアントには返さず、ここでサーバーログにのみ残す。
	if appErr.Reason == apperr.ReasonInternal && appErr.Err != nil {
		slog.Default().Error("internal error", slog.String("error", appErr.Err.Error()))
	}
	writeJSON(w, appErr.HTTPStatus, errorResponse{
		Reason:  string(appErr.Reason),
		Message: appErr.Message,
	})
}
