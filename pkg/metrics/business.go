package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 業務の数。「システムは動いているが、売れていない」「決済だけ失敗が増えた」といった、
// 技術のメトリクス(HTTP / gRPC / DB)だけでは見えない異常を見つけるためのもの。
// ラベルは、すべて、有限の固定の値(結果の種類など)。ユーザー ID・商品 ID・金額そのものは、ラベルに入れない。

// 注文の結果(pingu-api)。
const (
	OrderCreated           = "created"            // 新しく注文できた
	OrderReplayed          = "replayed"           // 同じ冪等性キーの再送。最初の結果を返した
	OrderOutOfStock        = "out_of_stock"       // 在庫が足りない
	OrderInsufficientPoint = "insufficient_point" // ポイントが足りない(在庫は戻した)
	OrderRejected          = "rejected"           // 入力の誤り・商品が無い・未ログインなど(呼び出し側の問題)
	OrderFailed            = "failed"             // サーバー側の失敗(在庫・決済・DB の失敗)
)

// 決済・返金・ポイント・在庫の結果(payment-api / orcan-api)。
const (
	ResultSucceeded    = "succeeded"
	ResultReplayed     = "replayed"
	ResultRejected     = "rejected"
	ResultInsufficient = "insufficient"
	ResultFailed       = "failed"
)

var (
	orders = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_orders_total",
		Help: "注文の数(結果別)。",
	}, []string{"result"})

	orderAmount = factory().NewCounter(prometheus.CounterOpts{
		Name: "pingucoin_order_amount_points_total",
		Help: "新しく成立した注文の、合計ポイント。",
	})

	authRequests = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_auth_requests_total",
		Help: "認証の結果(pingu-api)。anonymous = トークン無し、authenticated = 成功、invalid_token = 検証に失敗、profile_error = ユーザーの解決に失敗。",
	}, []string{"result"})

	payments = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_payments_total",
		Help: "決済の数(支払い方法・結果別)。",
	}, []string{"method", "result"})

	paymentAmount = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_payment_amount_total",
		Help: "成立した決済の合計額(通貨別)。",
	}, []string{"currency"})

	refunds = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_refunds_total",
		Help: "返金の数(結果別)。",
	}, []string{"result"})

	pointTransactions = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_point_transactions_total",
		Help: "ポイントの付与・消費の回数(種類・結果別)。type は credit / debit。",
	}, []string{"type", "result"})

	pointAmount = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_point_amount_total",
		Help: "ポイントの付与・消費の合計(種類別)。",
	}, []string{"type"})

	inventoryAdjustments = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_inventory_adjustments_total",
		Help: "在庫の増減の数(方向・結果別)。direction は increase / decrease。",
	}, []string{"direction", "result"})

	cacheRequests = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_cache_requests_total",
		Help: "キャッシュ(Redis)の参照の結果(キャッシュ名・結果別)。result は hit / miss / error。",
	}, []string{"cache", "result"})

	productsCreated = factory().NewCounter(prometheus.CounterOpts{
		Name: "pingucoin_products_created_total",
		Help: "出品(商品の作成)の数。",
	})

	blobOperations = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_blob_operations_total",
		Help: "Azure Blob Storage の操作の数(操作・結果別)。",
	}, []string{"operation", "result"})

	blobDuration = factory().NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_blob_operation_duration_seconds",
		Help:    "Azure Blob Storage の操作にかかった時間(秒)。",
		Buckets: LatencyBuckets,
	}, []string{"operation"})
)

// RecordOrder は、注文の結果を数える。成立した注文(OrderCreated)は、合計ポイントも足す。
func RecordOrder(result string, points int64) {
	orders.WithLabelValues(result).Inc()
	if result == OrderCreated && points > 0 {
		orderAmount.Add(float64(points))
	}
}

// RecordCache は、キャッシュの参照の結果(hit / miss / error)を数える。
func RecordCache(cache, result string) { cacheRequests.WithLabelValues(cache, result).Inc() }

// RecordAuth は、認証の結果を数える。
func RecordAuth(result string) { authRequests.WithLabelValues(result).Inc() }

// RecordPayment は、決済の結果を数える。成立(ResultSucceeded)したときは、金額も足す。
func RecordPayment(method, currency, result string, amount int64) {
	payments.WithLabelValues(method, result).Inc()
	if result == ResultSucceeded && amount > 0 {
		paymentAmount.WithLabelValues(currency).Add(float64(amount))
	}
}

// RecordRefund は、返金の結果を数える。
func RecordRefund(result string) { refunds.WithLabelValues(result).Inc() }

// RecordPointTransaction は、ポイントの付与(credit)・消費(debit)を数える。成功したときは、ポイントの合計も足す。
func RecordPointTransaction(txType, result string, amount int64) {
	pointTransactions.WithLabelValues(txType, result).Inc()
	if result == ResultSucceeded && amount > 0 {
		pointAmount.WithLabelValues(txType).Add(float64(amount))
	}
}

// RecordInventoryAdjustment は、在庫の増減を数える。amount の符号で、方向(increase / decrease)が決まる。
func RecordInventoryAdjustment(amount int32, result string) {
	direction := "increase"
	if amount < 0 {
		direction = "decrease"
	}
	inventoryAdjustments.WithLabelValues(direction, result).Inc()
}

// RecordProductCreated は、出品を数える。
func RecordProductCreated() { productsCreated.Inc() }

// ObserveBlob は、Blob Storage の操作の結果と時間を記録する。operation は固定の名前(upload_url / download_url / delete / ensure_containers)。
func ObserveBlob(operation string, start time.Time, err error) {
	result := ResultSucceeded
	if err != nil {
		result = ResultFailed
	}
	blobOperations.WithLabelValues(operation, result).Inc()
	blobDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
}

// ResultOf は、gRPC のハンドラーが返したエラーを、メトリクスの結果に分類する。
//   - nil                                                  ... succeeded
//   - 呼び出し側の問題(入力の誤り・存在しない・権限・前提条件)... rejected(アラートの対象にしない)
//   - それ以外(Internal・Unavailable・DeadlineExceeded など)... failed(サーバー側の失敗。アラートの対象)
func ResultOf(err error) string {
	switch status.Code(err) {
	case codes.OK:
		return ResultSucceeded
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied,
		codes.Unauthenticated, codes.FailedPrecondition, codes.OutOfRange:
		return ResultRejected
	default:
		return ResultFailed
	}
}

// Label は、クライアントが決められる文字列(支払い方法・通貨など)を、ラベルに使う前に、安全にする。
// 許可した値以外は、すべて "other" にまとめる(任意の文字列で、時系列を増やされないため)。
func Label(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return "other"
}
