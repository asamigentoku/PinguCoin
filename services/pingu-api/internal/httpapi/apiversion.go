package httpapi

import (
	"net/http"
	"strings"
)

// APIVersion は、このサーバーが公開している HTTP API(REST と GraphQL)のバージョン。URL の /api/v1/... の v1。
//
// バージョンの運用ルールは docs/VERSIONING.md を参照。要点:
//   - 同じバージョンの中では、互換性を保つ(項目の追加、新しいエンドポイントの追加は OK。削除・名前の変更・意味の変更は NG)。
//   - 互換性を壊す変更は、新しいバージョン(/api/v2)として出す。古いバージョンは、告知してから、期間を置いて止める。
const APIVersion = "v1"

// WithAPIVersion は、/api/ 以下のレスポンスに、X-API-Version ヘッダーを付ける(クライアントが、どのバージョンの
// API と話しているかを、レスポンスから確認できる。ログ・調査でも使う)。
func WithAPIVersion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, APIVersionPrefix+"/") {
			w.Header().Set("X-API-Version", APIVersion)
		}
		next.ServeHTTP(w, r)
	})
}
