// Package interceptor は orcan-api のgRPCサーバー用インターセプター(ミドルウェア)を集める。
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
//
// 各ハンドラー(internal/grpcserver配下)で個別にログを書く必要がないよう、
// ここ一箇所でリクエスト単位のログを一元的に出す。
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		response, err := handler(ctx, request)
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
			// codeがInternal等の場合、st.Messageはクライアント向けの汎用メッセージなので、
			// デバッグに必要な本当のエラー内容(DBエラー等)はerr.Error()からログに残す。
			attrs = append(attrs, slog.String("error", err.Error()))
			logger.LogAttrs(ctx, slog.LevelError, "grpc request failed", attrs...)
		}

		return response, err
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
