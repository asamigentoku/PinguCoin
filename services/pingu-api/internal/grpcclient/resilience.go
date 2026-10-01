package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/asamigentoku/PinguCoin/pkg/requestid"
)

// このファイルは、pingu-api が orcan-api / payment-api を呼ぶときの、リトライと期限(タイムアウト)の設定。
//
// 方針は「何でも再試行しない」こと。
//   - 再試行するのは、一時的な失敗(UNAVAILABLE = 接続できない・相手が起動中・切れた)だけ。
//     INVALID_ARGUMENT、NOT_FOUND、FAILED_PRECONDITION(在庫不足・ポイント不足)、PERMISSION_DENIED などは、
//     何度やっても同じなので、すぐにエラーを返す。DEADLINE_EXCEEDED(時間切れ)も、再試行しない
//     (相手が遅いときに、さらに負荷をかけて悪化させないため)。
//   - 再試行してよいのは、同じ呼び出しを2回やっても結果が変わらない(冪等な)メソッドだけ。
//     読み取りと、冪等性キーが付いている書き込み(在庫の消費、決済、ユーザーの作成)。
//     商品の作成・更新・ポイントの付与のような、2回やると結果が変わる書き込みは、再試行しない。
//   - 待ち時間は、指数バックオフ(100ms → 200ms、上限 1 秒)に、gRPC がジッター(ばらつき)を付ける。
//   - 障害で多くの呼び出しが失敗しているときは、再試行を控える(retryThrottling。再試行の集中を防ぐ)。
//   - 1回の呼び出しには、再試行も含めた全体の期限を付ける(呼び出し元が付けていなければ DefaultTimeout)。
//
// gRPC のリトライは、接続の層で動くので、呼び出しごとのインターセプター(認証トークンの付与など)は、再試行のたびに
// 呼び直されない。トークンは、最初の1回で付いたものが、そのまま再送される。

const (
	// DefaultTimeout は、1回の呼び出し(再試行を含む)の期限。呼び出し元の context に期限がなければ、これを付ける。
	DefaultTimeout = 10 * time.Second

	// MaxAttempts は、最初の1回も含めた、試行の最大回数。
	MaxAttempts = 3
)

type retryPolicy struct {
	MaxAttempts          int      `json:"maxAttempts"`
	InitialBackoff       string   `json:"initialBackoff"`
	MaxBackoff           string   `json:"maxBackoff"`
	BackoffMultiplier    float64  `json:"backoffMultiplier"`
	RetryableStatusCodes []string `json:"retryableStatusCodes"`
}

type methodName struct {
	Service string `json:"service"`
	Method  string `json:"method"`
}

type methodConfig struct {
	Name        []methodName `json:"name"`
	RetryPolicy retryPolicy  `json:"retryPolicy"`
}

type retryThrottling struct {
	MaxTokens  int     `json:"maxTokens"`
	TokenRatio float64 `json:"tokenRatio"`
}

type serviceConfig struct {
	MethodConfig    []methodConfig  `json:"methodConfig"`
	RetryThrottling retryThrottling `json:"retryThrottling"`
}

// ServiceConfig は、再試行してよいメソッド(生成コードの ..._FullMethodName、例 "/orcan.v1.ProductService/GetProduct")に、
// リトライの方針を付けた、gRPC の service config(JSON)を返す。
func ServiceConfig(retryableMethods []string) (string, error) {
	names := make([]methodName, 0, len(retryableMethods))
	for _, full := range retryableMethods {
		service, method, ok := splitFullMethod(full)
		if !ok {
			return "", fmt.Errorf("invalid gRPC method name %q (want \"/package.Service/Method\")", full)
		}
		names = append(names, methodName{Service: service, Method: method})
	}

	config := serviceConfig{
		MethodConfig: []methodConfig{{
			Name: names,
			RetryPolicy: retryPolicy{
				MaxAttempts:          MaxAttempts,
				InitialBackoff:       "0.1s",
				MaxBackoff:           "1s",
				BackoffMultiplier:    2,
				RetryableStatusCodes: []string{"UNAVAILABLE"},
			},
		}},
		// 失敗が続いて「トークン」が減ったら、再試行をやめる(成功すると少しずつ戻る)。
		RetryThrottling: retryThrottling{MaxTokens: 10, TokenRatio: 0.1},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func splitFullMethod(full string) (service, method string, ok bool) {
	trimmed := strings.TrimPrefix(full, "/")
	slash := strings.LastIndex(trimmed, "/")
	if slash <= 0 || slash == len(trimmed)-1 {
		return "", "", false
	}
	return trimmed[:slash], trimmed[slash+1:], true
}

// WithDefaultDeadline は、呼び出し元の context に期限がなければ、timeout の期限を付けるインターセプター。
// 相手が応答しないときに、いつまでも待ち続けない。期限は、再試行を含めた全体にかかる。
func WithDefaultDeadline(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline && timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ForwardRequestID は、context のリクエスト ID を、gRPC のメタデータ(x-request-id)にして、呼び出し先に渡す。
// orcan-api / payment-api が、同じ request_id でログを出すので、1つのリクエストを、サービスをまたいで追える(pkg/requestid)。
func ForwardRequestID() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id := requestid.FromContext(ctx); id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, requestid.MetadataKey, id)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ResilienceOptions は、gRPC クライアントに付ける、リトライと期限の設定(grpc.NewClient のオプション)。
func ResilienceOptions(retryableMethods []string, timeout time.Duration) ([]grpc.DialOption, error) {
	config, err := ServiceConfig(retryableMethods)
	if err != nil {
		return nil, err
	}
	return []grpc.DialOption{
		grpc.WithDefaultServiceConfig(config),
		grpc.WithChainUnaryInterceptor(WithDefaultDeadline(timeout), ForwardRequestID()),
	}, nil
}
