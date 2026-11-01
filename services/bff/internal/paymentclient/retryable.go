package paymentclient

import (
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

// RetryableMethods は、一時的な失敗(UNAVAILABLE)のときに、再試行してよい payment-api のメソッド。
// 生成コードの定数を使っているので、メソッド名の書き間違いは、ビルドエラーになる。
//
// お金が絡むので、厳しく絞っている。
//   - 読み取り(決済・返金・ポイントの取得)
//   - CreatePayment: 冪等性キーで、同じキーの再送は、二重に課金しない(payment-api が、同じ決済を返す)
//
// 次のものは、入れてはいけない(retryable_test.go が確かめる)。2回やると、2回付与・消費・返金される。
//
//	CancelPayment / RefundPayment / CreditPoints / DebitPoints
var RetryableMethods = []string{
	pb.PaymentService_GetPayment_FullMethodName,
	pb.PaymentService_ListPayments_FullMethodName,
	pb.PaymentService_ListRefunds_FullMethodName,
	pb.PaymentService_CreatePayment_FullMethodName,
	pb.PointService_GetPointAccount_FullMethodName,
	pb.PointService_ListPointTransactions_FullMethodName,
}
