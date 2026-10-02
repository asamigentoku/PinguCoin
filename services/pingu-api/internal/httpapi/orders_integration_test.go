package httpapi

import (
	"log/slog"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	orcanpb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/repository"
)

// これらは実際のPostgres(注文の記録用、TEST_DATABASE_URL)が必要。
// orcan-api(商品・在庫)とpayment-api(決済)は偽物に差し替え、注文APIが「何を、どの順で、どんなキーで呼ぶか」を確かめる。

type orderFixture struct {
	router    http.Handler
	inventory *fakeInventory
	payments  *fakePayments
	repo      *repository.OrderRepository
}

func newOrderFixture(t *testing.T) *orderFixture {
	t.Helper()
	db := testutil.NewDB(t, database.Migrate)
	f := &orderFixture{
		inventory: &fakeInventory{},
		payments:  &fakePayments{},
		repo:      repository.NewOrderRepository(db),
	}
	orcan := &orcanclient.Client{
		Product: &fakeProducts{products: map[uint32]*orcanpb.Product{
			10: {Id: 10, UserId: 99, Name: "Wallpaper", Price: 300},
			11: {Id: 11, UserId: 99, Name: "Template", Price: 500},
		}},
		Inventory: f.inventory,
	}
	handler := NewOrderHandler(slog.New(slog.DiscardHandler), orcan, &paymentclient.Client{Payment: f.payments}, f.repo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/orders", handler.CreateOrder)
	mux.HandleFunc("GET /api/v1/orders", handler.ListOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", handler.GetOrder)
	f.router = asUser(mux)
	return f
}

func (f *orderFixture) order(user, key, body string) (int, orderResponse) {
	recorder := call(f.router, "POST", "/api/v1/orders", callOptions{user: user, key: key, body: body})
	var order orderResponse
	_ = jsonUnmarshal(recorder.Body.Bytes(), &order)
	return recorder.Code, order
}

func TestPurchaseSucceeds(t *testing.T) {
	f := newOrderFixture(t)

	code, order := f.order("1", "key-1", `{"product_id":10}`)

	if code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", code)
	}
	if order.Status != "paid" || order.UserID != 1 || order.ProductID != 10 || order.Quantity != 1 || order.UnitPrice != 300 || order.TotalAmount != 300 {
		t.Errorf("unexpected order: %+v", order)
	}
	if order.PaymentID == 0 {
		t.Error("the order should reference the payment")
	}

	// 在庫の消費と決済は、同じ冪等性キーで、1回ずつ呼ばれる。決済は金額 = 単価 x 数量、通貨はPOINT、既定はポイント払い。
	if len(f.inventory.calls) != 1 || f.inventory.calls[0].GetAmount() != -1 || f.inventory.calls[0].GetIdempotencyKey() != "key-1" || f.inventory.calls[0].GetProductId() != 10 {
		t.Errorf("unexpected inventory calls: %+v", f.inventory.calls)
	}
	if len(f.payments.calls) != 1 {
		t.Fatalf("payment calls = %d, want 1", len(f.payments.calls))
	}
	payment := f.payments.calls[0]
	if payment.GetUserId() != 1 || payment.GetProductId() != 10 || payment.GetAmount() != 300 || payment.GetCurrency() != "POINT" ||
		payment.GetPaymentMethod() != "point" || payment.GetIdempotencyKey() != "key-1" {
		t.Errorf("unexpected payment request: %+v", payment)
	}
}

func TestQuantityMultipliesThePrice(t *testing.T) {
	f := newOrderFixture(t)

	code, order := f.order("1", "key-q", `{"product_id":11,"quantity":3}`)

	if code != http.StatusCreated || order.Quantity != 3 || order.TotalAmount != 1500 {
		t.Errorf("status=%d order=%+v; want total 1500", code, order)
	}
	if f.inventory.calls[0].GetAmount() != -3 || f.payments.calls[0].GetAmount() != 1500 {
		t.Errorf("inventory amount=%d payment amount=%d", f.inventory.calls[0].GetAmount(), f.payments.calls[0].GetAmount())
	}
}

// 同じキーの再送(リトライ・二重クリック)は、在庫も決済も新たに動かさず、最初の注文をそのまま返す。
func TestRepeatedKeyReturnsTheFirstOrderWithoutSideEffects(t *testing.T) {
	f := newOrderFixture(t)
	_, first := f.order("1", "key-1", `{"product_id":10}`)

	code, second := f.order("1", "key-1", `{"product_id":10}`)

	if code != http.StatusOK {
		t.Errorf("status = %d, want 200 for a replay", code)
	}
	if second.ID != first.ID {
		t.Errorf("a replay created a new order: %d vs %d", second.ID, first.ID)
	}
	if len(f.inventory.calls) != 1 || len(f.payments.calls) != 1 {
		t.Errorf("a replay touched the backends again: inventory=%d payments=%d", len(f.inventory.calls), len(f.payments.calls))
	}
}

// 在庫が足りないときは、決済に進まない(在庫のない商品に課金しない)。
func TestOutOfStockNeverReachesPayment(t *testing.T) {
	f := newOrderFixture(t)
	f.inventory.err = status.Error(codes.FailedPrecondition, "insufficient stock")

	code, _ := f.order("1", "key-1", `{"product_id":10}`)

	if code != http.StatusConflict {
		t.Errorf("status = %d, want 409", code)
	}
	if len(f.payments.calls) != 0 {
		t.Errorf("payment was attempted %d time(s) for an out-of-stock product", len(f.payments.calls))
	}
	if orders, _ := f.repo.FindByUser(1); len(orders) != 0 {
		t.Errorf("%d order(s) were recorded", len(orders))
	}
}

// 決済に失敗したら、消費した在庫を戻す。戻す操作は、消費とは別のキー(:release)を使う。
func TestFailedPaymentReleasesTheInventory(t *testing.T) {
	f := newOrderFixture(t)
	f.payments.err = status.Error(codes.FailedPrecondition, "insufficient points")

	code, _ := f.order("1", "key-1", `{"product_id":10,"quantity":2}`)

	if code != http.StatusConflict {
		t.Errorf("status = %d, want 409", code)
	}
	if len(f.inventory.calls) != 2 {
		t.Fatalf("inventory calls = %d, want 2 (consume, then release)", len(f.inventory.calls))
	}
	consume, release := f.inventory.calls[0], f.inventory.calls[1]
	if consume.GetAmount() != -2 || release.GetAmount() != 2 {
		t.Errorf("consume=%d release=%d; want -2 and +2", consume.GetAmount(), release.GetAmount())
	}
	if release.GetIdempotencyKey() != "key-1"+inventoryReleaseSuffix || release.GetIdempotencyKey() == consume.GetIdempotencyKey() {
		t.Errorf("release key = %q; it must differ from the consume key %q", release.GetIdempotencyKey(), consume.GetIdempotencyKey())
	}
	if orders, _ := f.repo.FindByUser(1); len(orders) != 0 {
		t.Errorf("%d order(s) were recorded for a failed payment", len(orders))
	}
}

func TestPendingPaymentIsRecordedAsPending(t *testing.T) {
	f := newOrderFixture(t)
	f.payments.status = "pending"

	_, order := f.order("1", "key-1", `{"product_id":10}`)

	if order.Status != "pending" {
		t.Errorf("status = %q, want pending", order.Status)
	}
}

func TestOrderInputIsValidated(t *testing.T) {
	f := newOrderFixture(t)
	tests := []struct {
		name string
		body string
		want int
	}{
		{"not JSON", `not json`, http.StatusBadRequest},
		{"no product", `{}`, http.StatusBadRequest},
		{"unknown product", `{"product_id":12345}`, http.StatusNotFound},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _ := f.order("1", "key-v"+string(rune('a'+i)), tt.body)
			if code != tt.want {
				t.Errorf("status = %d, want %d", code, tt.want)
			}
		})
	}
	if len(f.inventory.calls) != 0 || len(f.payments.calls) != 0 {
		t.Error("an invalid order touched the backends")
	}
}

func TestZeroOrNegativeQuantityDefaultsToOne(t *testing.T) {
	f := newOrderFixture(t)
	_, order := f.order("1", "key-z", `{"product_id":10,"quantity":-5}`)
	if order.Quantity != 1 || order.TotalAmount != 300 {
		t.Errorf("order = %+v; a non-positive quantity should be treated as 1", order)
	}
}

// 注文は本人にしか見えない。他人の注文は、存在を推測されないよう404にする。
func TestOrdersAreOnlyVisibleToTheirOwner(t *testing.T) {
	f := newOrderFixture(t)
	_, mine := f.order("1", "key-1", `{"product_id":10}`)
	f.order("2", "key-2", `{"product_id":11}`)

	list := decode[[]orderResponse](t, call(f.router, "GET", "/api/v1/orders", callOptions{user: "1"}))
	if len(list) != 1 || list[0].ID != mine.ID || list[0].UserID != 1 {
		t.Errorf("user 1 sees %+v", list)
	}

	own := call(f.router, "GET", "/api/v1/orders/"+itoa(mine.ID), callOptions{user: "1"})
	if own.Code != http.StatusOK {
		t.Errorf("own order: status = %d", own.Code)
	}
	other := call(f.router, "GET", "/api/v1/orders/"+itoa(mine.ID), callOptions{user: "2"})
	if other.Code != http.StatusNotFound {
		t.Errorf("someone else's order: status = %d, want 404", other.Code)
	}
	if got := call(f.router, "GET", "/api/v1/orders/abc", callOptions{user: "1"}); got.Code != http.StatusBadRequest {
		t.Errorf("invalid id: status = %d, want 400", got.Code)
	}
	if got := call(f.router, "GET", "/api/v1/orders/99999", callOptions{user: "1"}); got.Code != http.StatusNotFound {
		t.Errorf("missing order: status = %d, want 404", got.Code)
	}
}

func TestHasPaidOrder(t *testing.T) {
	f := newOrderFixture(t)
	f.order("1", "key-1", `{"product_id":10}`)

	for _, tt := range []struct {
		user, product uint
		want          bool
	}{{1, 10, true}, {1, 11, false}, {2, 10, false}} {
		got, err := f.repo.HasPaidOrder(tt.user, tt.product)
		if err != nil || got != tt.want {
			t.Errorf("HasPaidOrder(user %d, product %d) = %v, %v; want %v", tt.user, tt.product, got, err, tt.want)
		}
	}

	// 決済が保留中の注文では、購入済みとは扱わない(ダウンロードできない)。
	f.payments.status = "pending"
	f.order("3", "key-3", `{"product_id":10}`)
	if got, _ := f.repo.HasPaidOrder(3, 10); got {
		t.Error("a pending order was treated as purchased")
	}
}

// 注文の結果が、メトリクスに数えられる。成立した注文の合計ポイントも。
// 在庫不足・ポイント不足は、サーバーの失敗(failed)ではなく、通常の結果として、別のラベルで数える
// (failed を、アラートの対象にするので、混ぜない)。
func TestOrderOutcomesAreCounted(t *testing.T) {
	result := func(name string) float64 {
		return metricValue(t, "pingucoin_orders_total", map[string]string{"result": name})
	}
	amount := func() float64 { return metricValue(t, "pingucoin_order_amount_points_total", nil) }

	f := newOrderFixture(t)
	created, replayed, outOfStock, insufficient, rejected, failed := result("created"), result("replayed"), result("out_of_stock"), result("insufficient_point"), result("rejected"), result("failed")
	amountBefore := amount()

	f.order("1", "key-ok", `{"product_id":10,"quantity":2}`) // 300 x 2 = 600 ポイント
	f.order("1", "key-ok", `{"product_id":10,"quantity":2}`) // 同じキーの再送
	f.order("1", "key-bad", `{"product_id":0}`)              // 入力の誤り

	f.inventory.err = status.Error(codes.FailedPrecondition, "insufficient stock")
	f.order("2", "key-stock", `{"product_id":10}`)
	f.inventory.err = nil

	f.payments.err = status.Error(codes.FailedPrecondition, "insufficient points")
	f.order("2", "key-points", `{"product_id":10}`)
	f.payments.err = status.Error(codes.Internal, "payment backend down")
	f.order("2", "key-down", `{"product_id":10}`)

	for _, check := range []struct {
		name        string
		got, before float64
	}{
		{"created", result("created"), created},
		{"replayed", result("replayed"), replayed},
		{"out_of_stock", result("out_of_stock"), outOfStock},
		{"insufficient_point", result("insufficient_point"), insufficient},
		{"rejected", result("rejected"), rejected},
		{"failed", result("failed"), failed},
	} {
		if got := check.got - check.before; got != 1 {
			t.Errorf("%s increased by %v, want 1", check.name, got)
		}
	}
	if got := amount() - amountBefore; got != 600 {
		t.Errorf("order amount increased by %v, want 600 (replays and failures must not add)", got)
	}
}
