// Package requestid は、1つのリクエストを、複数のサービスをまたいで追いかけるための ID(リクエスト ID)を扱う。
//
// 流れ:
//
//		ブラウザ/Next.js ─ HTTP ヘッダー X-Request-Id ─▶ pingu-api ─ gRPC メタデータ x-request-id ─▶ orcan-api / payment-api
//
//	  - pingu-api は、リクエストに X-Request-Id があればそれを使い(なければ作って)、レスポンスにも付ける。
//	  - pingu-api が gRPC で呼ぶとき、同じ ID をメタデータで渡す。orcan-api / payment-api も、同じ ID をログに出す。
//	  - すべてのログに、同じ `request_id` が出るので、ログ管理(Datadog など)で、`request_id` で検索すれば、
//	    1つのリクエストが、どのサービスを通って、どこで失敗したかを、横断して追える。
//
// 外から来る ID は、そのままログに出すので、安全な文字だけ受け付ける(ログの偽装・改行の混入を防ぐ)。
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const (
	// Header は、HTTP のヘッダー名。
	Header = "X-Request-Id"
	// MetadataKey は、gRPC のメタデータのキー(小文字)。
	MetadataKey = "x-request-id"

	maxLength = 128
)

type contextKey struct{}

// New は、新しい ID(ランダムな 32 文字の 16 進数)を作る。
func New() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		// 乱数が読めないことは、まず無い。そのときも、空の ID にはしない。
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(buffer)
}

// Valid は、外から来た ID を、そのまま使ってよいか。英数字と . _ - だけで、1〜128 文字。
func Valid(id string) bool {
	if id == "" || len(id) > maxLength {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// OrNew は、外から来た ID が使えればそれを、使えなければ(空・長すぎる・変な文字)新しい ID を返す。
func OrNew(incoming string) string {
	if Valid(incoming) {
		return incoming
	}
	return New()
}

// WithContext は、context に ID を持たせる。
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext は、context の ID を返す。無ければ空文字。
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
