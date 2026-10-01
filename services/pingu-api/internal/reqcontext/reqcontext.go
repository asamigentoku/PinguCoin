// Package reqcontext はHTTPミドルウェア(internal/httpapi)で解決したログイン中ユーザー情報を、
// REST/GraphQLの両ハンドラーからcontext経由で参照できるようにする橋渡し。
package reqcontext

import "context"

// Claims はClerkのセッショントークン検証+orcan-apiのユーザープロフィール解決を経た、
// ログイン中ユーザーの情報。UserIDはorcan-api側のusers.id(既存のproducts/orders等が
// 参照しているuint32 IDとの互換性のため、Clerkの文字列IDではなくこちらを使う)。
type Claims struct {
	UserID      uint
	ClerkUserID string
	Email       string
	Name        string
}

type contextKey struct{}

var userContextKey = contextKey{}

// WithUser はログイン中ユーザー情報をcontextに積む。
func WithUser(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

// UserFromContext はcontextから認証済みユーザー情報を取り出す。未認証なら ok=false。
func UserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(userContextKey).(*Claims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}
