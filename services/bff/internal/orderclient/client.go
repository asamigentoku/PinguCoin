package orderclient

import (
	"context"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	sharedgrpc "github.com/asamigentoku/PinguCoin/services/bff/internal/grpcclient"
	pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	conn  *grpc.ClientConn
	Order pb.OrderServiceClient
}

func New(addr, token string) (*Client, error) {
	target, creds, e := sharedgrpc.Transport(addr)
	if e != nil {
		return nil, e
	}
	resilience, e := sharedgrpc.ResilienceOptions(RetryableMethods, sharedgrpc.DefaultTimeout)
	if e != nil {
		return nil, e
	}
	conn, e := grpc.NewClient(target, append(resilience, grpc.WithTransportCredentials(creds), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(metadata.AppendToOutgoingContext(ctx, "x-internal-token", token), method, req, reply, cc, opts...)
	}), grpc.WithChainUnaryInterceptor(metrics.UnaryClientInterceptor("order-api")), grpc.WithStatsHandler(metrics.ClientStatsHandler("order-api")))...)
	if e != nil {
		return nil, e
	}
	return &Client{conn: conn, Order: pb.NewOrderServiceClient(conn)}, nil
}
func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) HasPaidOrder(ctx context.Context, userID, productID uint64) (bool, error) {
	response, err := c.Order.HasPaidOrder(ctx, &pb.HasPaidOrderRequest{UserId: userID, ProductId: productID})
	if err != nil {
		return false, err
	}
	return response.GetPaid(), nil
}
