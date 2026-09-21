// Package paymentclient はpayment-apiへのgRPC呼び出しをラップする。
package paymentclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/payment/v1"
)

// Client はpayment-apiへのgRPCクライアントをまとめて持つ。
type Client struct {
	conn    *grpc.ClientConn
	Payment pb.PaymentServiceClient
}

// New はpayment-apiへのgRPCコネクションを1本張る。
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
