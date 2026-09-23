// Package paymentclient はpayment-apiへのgRPC呼び出しをラップする。
package paymentclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/payment/v1"
)

// internalTokenMetadataKey はpayment-api側(interceptor.Auth)が検証するメタデータのキーと揃える。
const internalTokenMetadataKey = "x-internal-token"

// Client はpayment-apiへのgRPCクライアントをまとめて持つ。
type Client struct {
	conn    *grpc.ClientConn
	Payment pb.PaymentServiceClient
}

// New はpayment-apiへのgRPCコネクションを1本張る。
// internalToken は全リクエストにサービス間認証用のメタデータとして付与する
// (payment-api側でこの値を検証し、pingu-api以外からの直接呼び出しを拒否する)。
func New(addr, internalToken string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(internalTokenInterceptor(internalToken)),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		Payment: pb.NewPaymentServiceClient(conn),
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
