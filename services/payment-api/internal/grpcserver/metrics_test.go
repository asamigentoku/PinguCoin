package grpcserver

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

// metricValue は、登録されているメトリクスから、名前とラベルが合うカウンターの値を読む(見つからなければ 0)。
func metricValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	metric:
		for _, metric := range family.GetMetric() {
			have := map[string]string{}
			for _, label := range metric.GetLabel() {
				have[label.GetName()] = label.GetValue()
			}
			for key, want := range labels {
				if have[key] != want {
					continue metric
				}
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}

// 決済の結果と金額が、メトリクスに数えられる。
//   - 成立した決済だけが、金額に足される(再送・ポイント不足・不正な入力は、足されない)
//   - 支払い方法は、クライアントが決められる文字列なので、既知の値以外は other にまとめる(時系列を、増やされない)
func TestPaymentMetrics(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)

	payments := func(method, result string) float64 {
		return metricValue(t, "pingucoin_payments_total", map[string]string{"method": method, "result": result})
	}
	amount := func() float64 {
		return metricValue(t, "pingucoin_payment_amount_total", map[string]string{"currency": "POINT"})
	}
	succeeded, replayed, insufficient, rejected, other := payments("point", "succeeded"), payments("point", "replayed"), payments("point", "insufficient"), payments("point", "rejected"), payments("other", "succeeded")
	amountBefore := amount()
	ctx := context.Background()

	f.payments.CreatePayment(ctx, pay(1, 400, "point", "k1"))       // 成立
	f.payments.CreatePayment(ctx, pay(1, 400, "point", "k1"))       // 同じキーの再送
	f.payments.CreatePayment(ctx, pay(1, 5000, "point", "k2"))      // ポイント不足
	f.payments.CreatePayment(ctx, pay(1, 0, "point", "k3"))         // 不正な入力
	f.payments.CreatePayment(ctx, pay(1, 100, "card-"+"xyz", "k4")) // 未知の支払い方法(ポイントには触れず、成立する)

	for _, check := range []struct {
		name        string
		got, before float64
	}{
		{"succeeded", payments("point", "succeeded"), succeeded},
		{"replayed", payments("point", "replayed"), replayed},
		{"insufficient", payments("point", "insufficient"), insufficient},
		{"rejected", payments("point", "rejected"), rejected},
		{"unknown method is merged into other", payments("other", "succeeded"), other},
	} {
		if got := check.got - check.before; got != 1 {
			t.Errorf("%s increased by %v, want 1", check.name, got)
		}
	}
	// 400(成立)+ 100(other の方法で成立)。再送・不足・不正な入力は、足されない。
	if got := amount() - amountBefore; got != 500 {
		t.Errorf("payment amount increased by %v, want 500", got)
	}
}

func TestPointAndRefundMetrics(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	credits := func() float64 {
		return metricValue(t, "pingucoin_point_amount_total", map[string]string{"type": "credit"})
	}
	debitFailed := func() float64 {
		return metricValue(t, "pingucoin_point_transactions_total", map[string]string{"type": "debit", "result": "rejected"})
	}
	refunds := func() float64 {
		return metricValue(t, "pingucoin_refunds_total", map[string]string{"result": "succeeded"})
	}
	creditsBefore, debitFailedBefore, refundsBefore := credits(), debitFailed(), refunds()

	f.credit(t, 7, 300)
	// 残高を超える消費は、失敗する(rejected。サーバーの失敗ではない)。
	if _, err := f.points.DebitPoints(ctx, &pb.DebitPointsRequest{UserId: 7, Amount: 9999, Reason: "too much"}); err == nil {
		t.Fatal("expected insufficient points")
	}
	response, err := f.payments.CreatePayment(ctx, pay(7, 200, "point", "refund-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: response.GetPayment().GetId(), Amount: 200, Reason: "test"}); err != nil {
		t.Fatal(err)
	}

	if got := credits() - creditsBefore; got != 300 {
		t.Errorf("credited points increased by %v, want 300", got)
	}
	if got := debitFailed() - debitFailedBefore; got != 1 {
		t.Errorf("rejected debits increased by %v, want 1", got)
	}
	if got := refunds() - refundsBefore; got != 1 {
		t.Errorf("refunds increased by %v, want 1", got)
	}
}
