package interceptor

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// internalTokenMetadataKey はbffがサービス間認証用の共有トークンを乗せるgRPCメタデータのキー。
const internalTokenMetadataKey = "x-internal-token"

// Auth はbff以外からの直接のgRPC呼び出しを拒否するための、サービス間認証インターセプター。
// エンドユーザーの認証(Clerkセッショントークンの検証)はbff側で完結しており、
// 呼び出される側(orcan-api / payment-api)はユーザーを識別しない。ここで検証するのは「呼び出し元がbffであること」のみ。
func Auth(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isHealthCheck(info.FullMethod) {
			return handler(ctx, request)
		}
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
