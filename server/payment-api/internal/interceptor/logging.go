// Package interceptor は payment-api のgRPCサーバー用インターセプター(ミドルウェア)を集める。
package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Logging は全RPC呼び出しを構造化ログに出す単項(unary)インターセプター。
// - 成功: Info
// - クライアント起因のエラー(NotFound/InvalidArgumentなど): Warn
// - サーバー起因のエラー(Internalなど): Error(元のエラー内容も含める)
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		st, _ := status.FromError(err)
		attrs := []slog.Attr{
			slog.String("method", info.FullMethod),
			slog.String("code", st.Code().String()),
			slog.Duration("duration", duration),
		}

		switch {
		case err == nil:
			logger.LogAttrs(ctx, slog.LevelInfo, "grpc request completed", attrs...)
		case isClientError(st.Code()):
			attrs = append(attrs, slog.String("message", st.Message()))
			logger.LogAttrs(ctx, slog.LevelWarn, "grpc request rejected", attrs...)
		default:
			attrs = append(attrs, slog.String("error", err.Error()))
			logger.LogAttrs(ctx, slog.LevelError, "grpc request failed", attrs...)
		}

		return resp, err
	}
}

func isClientError(code codes.Code) bool {
	switch code {
	case codes.NotFound, codes.InvalidArgument, codes.AlreadyExists, codes.Unauthenticated, codes.PermissionDenied, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
