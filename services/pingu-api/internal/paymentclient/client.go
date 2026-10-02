// Package paymentclient はpayment-apiへのgRPC呼び出しをラップする。
package paymentclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
	sharedgrpc "github.com/asamigentoku/PinguCoin/services/pingu-api/internal/grpcclient"
)

// internalTokenMetadataKey はpayment-api側(interceptor.Auth)が検証するメタデータのキーと揃える。
const internalTokenMetadataKey = "x-internal-token"

// Client はpayment-apiへのgRPCクライアントをまとめて持つ。
type Client struct {
	conn    *grpc.ClientConn
	Payment pb.PaymentServiceClient
	Point   pb.PointServiceClient
}

// New はpayment-apiへのgRPCコネクションを1本張る。
// internalToken は全リクエストにサービス間認証用のメタデータとして付与する
// (payment-api側でこの値を検証し、pingu-api以外からの直接呼び出しを拒否する)。
func New(addr, internalToken string) (*Client, error) {
	target, transportCredentials, err := sharedgrpc.Transport(addr)
	if err != nil {
		return nil, err
	}

	// 一時的な失敗のときのリトライ(冪等なメソッドだけ)と、呼び出しの期限。
	resilience, err := sharedgrpc.ResilienceOptions(RetryableMethods, sharedgrpc.DefaultTimeout)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(target, append(resilience,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithUnaryInterceptor(internalTokenInterceptor(internalToken)),
		// メトリクス: 呼び出しの数・結果・時間(UnaryClientInterceptor)と、実際に送った回数(再試行を含む。ClientStatsHandler)。
		grpc.WithChainUnaryInterceptor(metrics.UnaryClientInterceptor("payment-api")),
		grpc.WithStatsHandler(metrics.ClientStatsHandler("payment-api")),
	)...)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		Payment: pb.NewPaymentServiceClient(conn),
		Point:   pb.NewPointServiceClient(conn),
	}, nil
}

func (client *Client) Close() error {
	return client.conn.Close()
}

func internalTokenInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, internalTokenMetadataKey, token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
