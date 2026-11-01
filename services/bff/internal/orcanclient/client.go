// Package orcanclient はmedia-apiへのgRPC呼び出しをラップする。
// bffのREST層はこのクライアント越しにのみmedia-apiへアクセスする。
package orcanclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
	sharedgrpc "github.com/asamigentoku/PinguCoin/services/bff/internal/grpcclient"
)

// internalTokenMetadataKey はmedia-api側(interceptor.Auth)が検証するメタデータのキーと揃える。
const internalTokenMetadataKey = "x-internal-token"

// Client はmedia-apiの各サービスへのgRPCクライアントをまとめて持つ。
type Client struct {
	conn      *grpc.ClientConn
	Product   pb.ProductServiceClient
	Asset     pb.ProductAssetServiceClient
	Inventory pb.ProductInventoryServiceClient
	User      pb.UserServiceClient
}

// New はmedia-apiへのgRPCコネクションを1本張り、各サービスのクライアントを作る。
// internalToken は全リクエストにサービス間認証用のメタデータとして付与する
// (media-api側でこの値を検証し、bff以外からの直接呼び出しを拒否する)。
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
		grpc.WithChainUnaryInterceptor(metrics.UnaryClientInterceptor("media-api")),
		grpc.WithStatsHandler(metrics.ClientStatsHandler("media-api")),
	)...)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:      conn,
		Product:   pb.NewProductServiceClient(conn),
		Asset:     pb.NewProductAssetServiceClient(conn),
		Inventory: pb.NewProductInventoryServiceClient(conn),
		User:      pb.NewUserServiceClient(conn),
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
