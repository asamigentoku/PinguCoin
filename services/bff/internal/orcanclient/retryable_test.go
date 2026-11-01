package orcanclient

import (
	"strings"
	"testing"
)

// 再試行してよいのは、冪等なメソッドだけ。2回やると結果が変わる書き込みが、うっかり入らないようにする番人。
func TestRetryableMethodsAreOnlyIdempotentOnes(t *testing.T) {
	if len(RetryableMethods) == 0 {
		t.Fatal("no retryable methods")
	}

	// 冪等性キー(または find-or-create)があるので、書き込みでも再試行してよいもの。
	allowedWrites := map[string]string{
		"AdjustProductInventory": "idempotency_key で、同じキーの再送は二重に減らさない",
		"EnsureUser":             "Clerk の ID で、あれば返し、なければ作る",
	}
	unsafePrefixes := []string{"Create", "Update", "Delete", "Confirm", "Adjust", "Ensure", "Credit", "Debit", "Refund", "Cancel"}

	seen := map[string]bool{}
	for _, full := range RetryableMethods {
		if seen[full] {
			t.Errorf("%s is listed twice", full)
		}
		seen[full] = true

		method := full[strings.LastIndex(full, "/")+1:]
		if _, ok := allowedWrites[method]; ok {
			continue
		}
		for _, prefix := range unsafePrefixes {
			if strings.HasPrefix(method, prefix) {
				t.Errorf("%s looks like a write that changes the result when repeated; it must not be retried", full)
			}
		}
	}
	for method := range allowedWrites {
		found := false
		for _, full := range RetryableMethods {
			found = found || strings.HasSuffix(full, "/"+method)
		}
		if !found {
			t.Errorf("%s should be retryable (it is idempotent), but is not listed", method)
		}
	}
}
