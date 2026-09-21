// Package orcanclient はorcan-apiへのgRPC呼び出しをラップする。
// pingu-apiのGraphQL/REST層はこのクライアント越しにのみorcan-apiへアクセスし、
// gRPCの型(pb.*)をGraphQL/REST層に漏らさない。
package orcanclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
)

// Client はorcan-apiの各サービスへのgRPCクライアントをまとめて持つ。
type Client struct {
	conn    *grpc.ClientConn
	Product pb.ProductServiceClient
	User    pb.UserServiceClient
}

// New はorcan-apiへのgRPCコネクションを1本張り、各サービスのクライアントを作る。
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		Product: pb.NewProductServiceClient(conn),
		User:    pb.NewUserServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
