package paymentclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// pingu-api が payment-api を呼ぶとき、サービス間認証用のトークンを、必ずメタデータに付ける。
// 付かないと、payment-api に、すべての呼び出しを拒否される。
func TestInternalTokenIsAttachedToEveryCall(t *testing.T) {
	var sent metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		sent, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}

	err := internalTokenInterceptor("shared-secret")(context.Background(), "/payment.v1.PaymentService/CreatePayment", nil, nil, nil, invoker)

	if err != nil {
		t.Fatal(err)
	}
	if got := sent.Get(internalTokenMetadataKey); len(got) != 1 || got[0] != "shared-secret" {
		t.Errorf("metadata %q = %v, want [shared-secret]", internalTokenMetadataKey, got)
	}
}

// 呼び出し元がすでに付けているメタデータは、消さない。
func TestExistingMetadataIsKept(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-request-id", "abc")
	var sent metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		sent, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}

	_ = internalTokenInterceptor("t")(ctx, "/x", nil, nil, nil, invoker)

	if got := sent.Get("x-request-id"); len(got) != 1 || got[0] != "abc" {
		t.Errorf("x-request-id = %v, want [abc]", got)
	}
}

// 無効なアドレスでは、クライアントを作らない。
func TestNewRejectsAnInvalidAddress(t *testing.T) {
	if _, err := New("", "token"); err == nil {
		t.Error("expected an error for an empty address")
	}
}

func TestNewBuildsAllServiceClients(t *testing.T) {
	client, err := New("localhost:8081", "token")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if client.Payment == nil || client.Point == nil {
		t.Errorf("a service client is missing: %+v", client)
	}
}
