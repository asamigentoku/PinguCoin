package orderclient

import (
	"testing"

	pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"
)

func TestCreateOrderIsRetryableBecauseItIsIdempotent(t *testing.T) {
	for _, method := range RetryableMethods {
		if method == pb.OrderService_CreateOrder_FullMethodName {
			return
		}
	}
	t.Fatal("CreateOrder must be retryable because its idempotency key prevents duplicate orders")
}
