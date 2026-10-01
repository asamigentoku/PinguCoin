// Package interceptor は gRPC サーバー(orcan-api / payment-api)共通のインターセプター(ミドルウェア)を集める。
package interceptor

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/requestid"
)

// RequestID は、呼び出し元(pingu-api)から渡されたリクエスト ID(メタデータ x-request-id)を、context に持たせる。
// 渡されていなければ(または、使えない文字を含んでいれば)新しく作る。Logging より前に置くこと。
// これで、1つのリクエストを、pingu-api と orcan-api / payment-api をまたいで、同じ request_id で追える。
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incoming := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(requestid.MetadataKey); len(values) > 0 {
				incoming = values[0]
			}
		}
		return handler(requestid.WithContext(ctx, requestid.OrNew(incoming)), request)
	}
}

// Logging は全RPC呼び出しを構造化ログに出す単項(unary)インターセプター。
// - 成功: info
// - クライアント起因のエラー(NotFound/InvalidArgumentなど): warn
// - サーバー起因のエラー(Internalなど): error(元のエラー内容も含める)
//
// ログの属性は、Datadog の標準属性にそろえる(pkg/logging を参照)。
//
//	rpc.system / rpc.service / rpc.method / rpc.grpc.status_code,  duration(ナノ秒),  request_id,  error.kind / error.message
//
// 各ハンドラー(internal/grpcserver配下)で個別にログを書く必要がないよう、
// ここ一箇所でリクエスト単位のログを一元的に出す。
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isHealthCheck(info.FullMethod) {
			return handler(ctx, request)
		}
		start := time.Now()
		response, err := handler(ctx, request)
		duration := time.Since(start)

		st, _ := status.FromError(err)
		service, method := splitFullMethod(info.FullMethod)
		attrs := []slog.Attr{
			slog.Group("rpc",
				slog.String("system", "grpc"),
				slog.String("service", service),
				slog.String("method", method),
				slog.Group("grpc", slog.String("status_code", st.Code().String())),
			),
			logging.Duration(duration),
			logging.RequestID(ctx),
		}

		switch {
		case err == nil:
			logger.LogAttrs(ctx, slog.LevelInfo, "grpc request completed", attrs...)
		case isClientError(st.Code()):
			attrs = append(attrs, slog.Group("error", slog.String("kind", st.Code().String()), slog.String("message", st.Message())))
			logger.LogAttrs(ctx, slog.LevelWarn, "grpc request rejected", attrs...)
		default:
			// codeがInternal等の場合、st.Messageはクライアント向けの汎用メッセージなので、
			// デバッグに必要な本当のエラー内容(DBエラー等)はerr.Error()からログに残す。
			attrs = append(attrs, logging.Err(err))
			logger.LogAttrs(ctx, slog.LevelError, "grpc request failed", attrs...)
		}

		return response, err
	}
}

// splitFullMethod は "/orcan.v1.ProductService/GetProduct" を、サービス名とメソッド名に分ける。
func splitFullMethod(full string) (service, method string) {
	trimmed := strings.TrimPrefix(full, "/")
	if slash := strings.LastIndex(trimmed, "/"); slash >= 0 {
		return trimmed[:slash], trimmed[slash+1:]
	}
	return "", trimmed
}

func isClientError(code codes.Code) bool {
	switch code {
	case codes.NotFound, codes.InvalidArgument, codes.AlreadyExists, codes.Unauthenticated, codes.PermissionDenied, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
