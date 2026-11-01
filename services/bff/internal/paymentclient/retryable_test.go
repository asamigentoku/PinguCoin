package paymentclient

import (
	"strings"
	"testing"
)

// お金が絡むので、再試行してよいメソッドを、厳しく絞っている。
// ポイントの付与・消費、返金、キャンセルは、2回やると結果が変わるので、再試行してはいけない。
func TestRetryableMethodsNeverIncludeMoneyMovingOperations(t *testing.T) {
	if len(RetryableMethods) == 0 {
		t.Fatal("no retryable methods")
	}
	forbidden := []string{"CancelPayment", "RefundPayment", "CreditPoints", "DebitPoints"}

	for _, full := range RetryableMethods {
		for _, name := range forbidden {
			if strings.HasSuffix(full, "/"+name) {
				t.Errorf("%s moves money and is not safe to repeat; it must not be retried", full)
			}
		}
	}
}

// CreatePayment は、冪等性キーで、同じキーの再送は二重に課金しない(payment-api が、同じ決済を返す)ので、再試行してよい。
func TestCreatePaymentIsRetryableBecauseItIsIdempotent(t *testing.T) {
	for _, full := range RetryableMethods {
		if strings.HasSuffix(full, "/CreatePayment") {
			return
		}
	}
	t.Error("CreatePayment is idempotent (idempotency_key) and should be retryable")
}
