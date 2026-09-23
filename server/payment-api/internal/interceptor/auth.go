package interceptor

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// internalTokenMetadataKey はpingu-apiがサービス間認証用の共有トークンを乗せるgRPCメタデータのキー。
const internalTokenMetadataKey = "x-internal-token"

// Auth はpingu-api以外からの直接のgRPC呼び出しを拒否するための、サービス間認証インターセプター。
// エンドユーザーの認証(Clerkセッショントークンの検証)はpingu-api側で完結しており、
// payment-apiはユーザーを識別しない。ここで検証するのは「呼び出し元がpingu-apiであること」のみ。
func Auth(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !hasValidToken(ctx, token) {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid internal token")
		}
		return handler(ctx, request)
	}
}

func hasValidToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get(internalTokenMetadataKey)
	if len(values) == 0 {
		return false
	}
	// タイミング攻撃でトークンを推測されないよう定数時間で比較する。
	return subtle.ConstantTimeCompare([]byte(values[0]), []byte(token)) == 1
}
