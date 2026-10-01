package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orcanpb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	paymentpb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/reqcontext"
)

// このファイルは、orcan-api / payment-api の代わりをする偽のgRPCクライアント。
// 実物のクライアントはインターフェースなので、必要なメソッドだけ実装した構造体に差し替えられる
// (実装していないメソッドを呼ぶと、埋め込んだnilのインターフェース経由でパニックする=想定外の呼び出しに気づける)。

type fakeProducts struct {
	orcanpb.ProductServiceClient
	products map[uint32]*orcanpb.Product
}

func (f *fakeProducts) GetProduct(_ context.Context, in *orcanpb.GetProductRequest, _ ...grpc.CallOption) (*orcanpb.GetProductResponse, error) {
	product, ok := f.products[in.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return &orcanpb.GetProductResponse{Product: product}, nil
}

type fakeInventory struct {
	orcanpb.ProductInventoryServiceClient
	mu    sync.Mutex
	calls []*orcanpb.AdjustProductInventoryRequest
	err   error
}

func (f *fakeInventory) AdjustProductInventory(_ context.Context, in *orcanpb.AdjustProductInventoryRequest, _ ...grpc.CallOption) (*orcanpb.AdjustProductInventoryResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	// 消費(負の数)だけ失敗させられる。在庫を戻す操作(正の数)は常に成功する。
	if f.err != nil && in.GetAmount() < 0 {
		return nil, f.err
	}
	return &orcanpb.AdjustProductInventoryResponse{}, nil
}

type fakePayments struct {
	paymentpb.PaymentServiceClient
	mu     sync.Mutex
	calls  []*paymentpb.CreatePaymentRequest
	err    error
	status string // 空なら "succeeded"
}

func (f *fakePayments) CreatePayment(_ context.Context, in *paymentpb.CreatePaymentRequest, _ ...grpc.CallOption) (*paymentpb.CreatePaymentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	paymentStatus := f.status
	if paymentStatus == "" {
		paymentStatus = "succeeded"
	}
	return &paymentpb.CreatePaymentResponse{Payment: &paymentpb.Payment{Id: uint32(100 + len(f.calls)), Status: paymentStatus}}, nil
}

// fakePoints は payment-api のポイントの実装と同じ動きをする(履歴は新しい順=id降順で返す)。
type fakePoints struct {
	paymentpb.PointServiceClient
	mu           sync.Mutex
	balance      map[uint32]int64
	transactions map[uint32][]*paymentpb.PointTransaction // 古い順に保持
	credits      []*paymentpb.CreditPointsRequest
}

func newFakePoints() *fakePoints {
	return &fakePoints{balance: map[uint32]int64{}, transactions: map[uint32][]*paymentpb.PointTransaction{}}
}

func (f *fakePoints) ListPointTransactions(_ context.Context, in *paymentpb.ListPointTransactionsRequest, _ ...grpc.CallOption) (*paymentpb.ListPointTransactionsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	history := f.transactions[in.GetUserId()]
	newestFirst := make([]*paymentpb.PointTransaction, 0, len(history))
	for i := len(history) - 1; i >= 0; i-- {
		newestFirst = append(newestFirst, history[i])
	}
	return &paymentpb.ListPointTransactionsResponse{Transactions: newestFirst}, nil
}

func (f *fakePoints) CreditPoints(_ context.Context, in *paymentpb.CreditPointsRequest, _ ...grpc.CallOption) (*paymentpb.CreditPointsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.credits = append(f.credits, in)
	f.balance[in.GetUserId()] += in.GetAmount()
	transaction := &paymentpb.PointTransaction{
		Id: uint32(len(f.transactions[in.GetUserId()]) + 1), UserId: in.GetUserId(), Amount: in.GetAmount(),
		Type: "credit", Reason: in.GetReason(), BalanceAfter: f.balance[in.GetUserId()],
	}
	f.transactions[in.GetUserId()] = append(f.transactions[in.GetUserId()], transaction)
	return &paymentpb.CreditPointsResponse{Transaction: transaction}, nil
}

func (f *fakePoints) GetPointAccount(_ context.Context, in *paymentpb.GetPointAccountRequest, _ ...grpc.CallOption) (*paymentpb.GetPointAccountResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &paymentpb.GetPointAccountResponse{Account: &paymentpb.PointAccount{UserId: in.GetUserId(), Balance: f.balance[in.GetUserId()]}}, nil
}

// asUser は「ログイン済みのユーザー」としてリクエストを流し込むためのラッパー。
// 本物のミドルウェア(WithOptionalAuth)は、ClerkとDBが必要なので、ここではヘッダーからユーザーを決める。
// X-Test-User が無ければ未ログインとして扱う。
func asUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get("X-Test-User"); id != "" {
			userID, _ := strconv.Atoi(id)
			r = r.WithContext(reqcontext.WithUser(r.Context(), &reqcontext.Claims{UserID: uint(userID), ClerkUserID: "clerk_" + id}))
		}
		next.ServeHTTP(w, r)
	})
}

type callOptions struct {
	user string // 空なら未ログイン
	key  string // Idempotency-Key
	body string
}

func call(handler http.Handler, method, path string, options callOptions) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(options.body))
	if options.user != "" {
		request.Header.Set("X-Test-User", options.user)
	}
	if options.key != "" {
		request.Header.Set(idempotencyKeyHeader, options.key)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }
