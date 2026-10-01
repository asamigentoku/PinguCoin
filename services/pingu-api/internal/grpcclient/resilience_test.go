package grpcclient_test

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/asamigentoku/PinguCoin/pkg/requestid"
	orcanv1 "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	paymentv1 "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/grpcclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
)

// 偽のサーバーに、「最初の N 回は、このエラーを返す」と台本を書いて、リトライの動きを確かめる。
type script struct {
	mu         sync.Mutex
	calls      map[string]int
	tokens     map[string][]string // 呼び出しごとに受け取った認証トークン
	requestIDs map[string][]string // 呼び出しごとに受け取ったリクエスト ID
	failures   map[string]failure
}

type failure struct {
	code  codes.Code
	times int  // 最初の何回、失敗するか(-1 なら、ずっと)
	block bool // 失敗せず、期限が切れるまで応答しない
}

func newScript() *script {
	return &script{calls: map[string]int{}, tokens: map[string][]string{}, requestIDs: map[string][]string{}, failures: map[string]failure{}}
}

func (s *script) failFirst(method string, code codes.Code, times int) {
	s.failures[method] = failure{code: code, times: times}
}
func (s *script) blockForever(method string) { s.failures[method] = failure{block: true} }

func (s *script) callCount(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[method]
}

func (s *script) attempt(ctx context.Context, method string) error {
	s.mu.Lock()
	s.calls[method]++
	attempt := s.calls[method]
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.tokens[method] = append(s.tokens[method], md.Get("x-internal-token")...)
		s.requestIDs[method] = append(s.requestIDs[method], md.Get("x-request-id")...)
	}
	f, scripted := s.failures[method]
	s.mu.Unlock()

	switch {
	case !scripted:
		return nil
	case f.block:
		<-ctx.Done()
		return status.FromContextError(ctx.Err()).Err()
	case f.times < 0 || attempt <= f.times:
		return status.Error(f.code, "scripted "+f.code.String())
	}
	return nil
}

type fakeProducts struct {
	orcanv1.UnimplementedProductServiceServer
	s *script
}

func (f *fakeProducts) GetProduct(ctx context.Context, _ *orcanv1.GetProductRequest) (*orcanv1.GetProductResponse, error) {
	if err := f.s.attempt(ctx, "GetProduct"); err != nil {
		return nil, err
	}
	return &orcanv1.GetProductResponse{Product: &orcanv1.Product{Id: 1}}, nil
}

func (f *fakeProducts) CreateProduct(ctx context.Context, _ *orcanv1.CreateProductRequest) (*orcanv1.CreateProductResponse, error) {
	if err := f.s.attempt(ctx, "CreateProduct"); err != nil {
		return nil, err
	}
	return &orcanv1.CreateProductResponse{Product: &orcanv1.Product{Id: 1}}, nil
}

type fakeInventory struct {
	orcanv1.UnimplementedProductInventoryServiceServer
	s *script
}

func (f *fakeInventory) AdjustProductInventory(ctx context.Context, _ *orcanv1.AdjustProductInventoryRequest) (*orcanv1.AdjustProductInventoryResponse, error) {
	if err := f.s.attempt(ctx, "AdjustProductInventory"); err != nil {
		return nil, err
	}
	return &orcanv1.AdjustProductInventoryResponse{}, nil
}

type fakePoints struct {
	paymentv1.UnimplementedPointServiceServer
	s *script
}

func (f *fakePoints) CreditPoints(ctx context.Context, _ *paymentv1.CreditPointsRequest) (*paymentv1.CreditPointsResponse, error) {
	if err := f.s.attempt(ctx, "CreditPoints"); err != nil {
		return nil, err
	}
	return &paymentv1.CreditPointsResponse{}, nil
}

func (f *fakePoints) GetPointAccount(ctx context.Context, _ *paymentv1.GetPointAccountRequest) (*paymentv1.GetPointAccountResponse, error) {
	if err := f.s.attempt(ctx, "GetPointAccount"); err != nil {
		return nil, err
	}
	return &paymentv1.GetPointAccountResponse{Account: &paymentv1.PointAccount{UserId: 1}}, nil
}

type clients struct {
	products  orcanv1.ProductServiceClient
	inventory orcanv1.ProductInventoryServiceClient
	points    paymentv1.PointServiceClient
}

// start は、偽のサーバーをメモリ上(bufconn)で動かして、本物と同じリトライ設定(orcanclient / paymentclient の一覧)を
// 付けたクライアントを返す。認証トークンを付ける処理も付けて、再試行でも付き続けることを確かめる。
func start(t *testing.T, s *script, timeout time.Duration) clients {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	orcanv1.RegisterProductServiceServer(server, &fakeProducts{s: s})
	orcanv1.RegisterProductInventoryServiceServer(server, &fakeInventory{s: s})
	paymentv1.RegisterPointServiceServer(server, &fakePoints{s: s})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	methods := append(append([]string{}, orcanclient.RetryableMethods...), paymentclient.RetryableMethods...)
	resilience, err := grpcclient.ResilienceOptions(methods, timeout)
	if err != nil {
		t.Fatal(err)
	}
	token := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(metadata.AppendToOutgoingContext(ctx, "x-internal-token", "secret"), method, req, reply, cc, opts...)
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", append(resilience,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(token),
	)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return clients{products: orcanv1.NewProductServiceClient(conn), inventory: orcanv1.NewProductInventoryServiceClient(conn), points: paymentv1.NewPointServiceClient(conn)}
}

func TestReadsAreRetriedOnUnavailable(t *testing.T) {
	s := newScript()
	s.failFirst("GetProduct", codes.Unavailable, 2)
	c := start(t, s, 5*time.Second)

	response, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1})

	if err != nil || response.GetProduct().GetId() != 1 {
		t.Fatalf("err=%v response=%v; the third attempt should succeed", err, response)
	}
	if got := s.callCount("GetProduct"); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

// 無限には再試行しない。最大 MaxAttempts 回で諦める。
func TestGivesUpAfterMaxAttempts(t *testing.T) {
	s := newScript()
	s.failFirst("GetProduct", codes.Unavailable, -1)
	c := start(t, s, 5*time.Second)
	begin := time.Now()

	_, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1})

	if status.Code(err) != codes.Unavailable {
		t.Errorf("code = %v, want Unavailable", status.Code(err))
	}
	if got := s.callCount("GetProduct"); got != grpcclient.MaxAttempts {
		t.Errorf("attempts = %d, want %d", got, grpcclient.MaxAttempts)
	}
	if elapsed := time.Since(begin); elapsed > 3*time.Second {
		t.Errorf("took %v; the backoff should be short (<= 1s per wait)", elapsed)
	}
}

// 何度やっても同じ結果になるエラー(入力の間違い、在庫不足など)は、再試行しない。
func TestPermanentErrorsAreNotRetried(t *testing.T) {
	for _, code := range []codes.Code{
		codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition, codes.PermissionDenied,
		codes.Unauthenticated, codes.AlreadyExists, codes.Internal,
	} {
		t.Run(code.String(), func(t *testing.T) {
			s := newScript()
			s.failFirst("GetProduct", code, -1)
			c := start(t, s, 5*time.Second)

			_, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1})

			if status.Code(err) != code {
				t.Errorf("code = %v, want %v", status.Code(err), code)
			}
			if got := s.callCount("GetProduct"); got != 1 {
				t.Errorf("attempts = %d, want 1 (no retry for %v)", got, code)
			}
		})
	}
}

// 2回やると結果が変わる書き込み(商品の作成、ポイントの付与)は、UNAVAILABLE でも再試行しない。
// 1回目が実は成功していたら、二重に作られる・二重に付与されるため。
func TestNonIdempotentWritesAreNeverRetried(t *testing.T) {
	s := newScript()
	s.failFirst("CreateProduct", codes.Unavailable, -1)
	s.failFirst("CreditPoints", codes.Unavailable, -1)
	c := start(t, s, 5*time.Second)

	if _, err := c.products.CreateProduct(context.Background(), &orcanv1.CreateProductRequest{Name: "x"}); status.Code(err) != codes.Unavailable {
		t.Errorf("CreateProduct: code = %v", status.Code(err))
	}
	if _, err := c.points.CreditPoints(context.Background(), &paymentv1.CreditPointsRequest{UserId: 1, Amount: 10, Reason: "x"}); status.Code(err) != codes.Unavailable {
		t.Errorf("CreditPoints: code = %v", status.Code(err))
	}
	if got := s.callCount("CreateProduct"); got != 1 {
		t.Errorf("CreateProduct attempts = %d, want 1", got)
	}
	if got := s.callCount("CreditPoints"); got != 1 {
		t.Errorf("CreditPoints attempts = %d, want 1", got)
	}
}

// 冪等性キーのある書き込み(在庫の消費)は、再試行してよい(同じキーなら、二重に減らない)。
func TestIdempotentWritesAreRetried(t *testing.T) {
	s := newScript()
	s.failFirst("AdjustProductInventory", codes.Unavailable, 1)
	c := start(t, s, 5*time.Second)

	_, err := c.inventory.AdjustProductInventory(context.Background(), &orcanv1.AdjustProductInventoryRequest{ProductId: 1, Amount: -1, IdempotencyKey: "k"})

	if err != nil {
		t.Fatal(err)
	}
	if got := s.callCount("AdjustProductInventory"); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

// 再試行のたびに、サービス間認証のトークンも、ちゃんと送られる(送られないと、全部拒否される)。
func TestTheInternalTokenIsSentOnEveryAttempt(t *testing.T) {
	s := newScript()
	s.failFirst("GetProduct", codes.Unavailable, 2)
	c := start(t, s, 5*time.Second)

	if _, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1}); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if got := s.tokens["GetProduct"]; len(got) != 3 || got[0] != "secret" || got[1] != "secret" || got[2] != "secret" {
		t.Errorf("tokens received per attempt = %v, want [secret secret secret]", got)
	}
}

// 相手が応答しなければ、期限(timeout)で諦める。時間切れ(DEADLINE_EXCEEDED)は、再試行しない
// (遅いときに、さらに負荷をかけて悪化させない)。
func TestSlowCallsTimeOutAndAreNotRetried(t *testing.T) {
	s := newScript()
	s.blockForever("GetProduct")
	c := start(t, s, 150*time.Millisecond)
	begin := time.Now()

	_, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1})

	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("code = %v, want DeadlineExceeded", status.Code(err))
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Errorf("took %v; the default deadline did not apply", elapsed)
	}
	if got := s.callCount("GetProduct"); got != 1 {
		t.Errorf("attempts = %d, want 1 (a timeout is not retried)", got)
	}
}

// 呼び出し元が付けた期限は、上書きしない。
func TestCallerDeadlineIsRespected(t *testing.T) {
	s := newScript()
	s.blockForever("GetProduct")
	c := start(t, s, 30*time.Second) // 既定の期限は長いが、呼び出し元が 100ms と決めている
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()

	_, err := c.products.GetProduct(ctx, &orcanv1.GetProductRequest{Id: 1})

	if status.Code(err) != codes.DeadlineExceeded || time.Since(begin) > 3*time.Second {
		t.Errorf("code=%v after %v; the caller's 100ms deadline should win", status.Code(err), time.Since(begin))
	}
}

func TestWithDefaultDeadline(t *testing.T) {
	var seen time.Time
	var had bool
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		seen, had = ctx.Deadline()
		return nil
	}

	_ = grpcclient.WithDefaultDeadline(2*time.Second)(context.Background(), "/x/y", nil, nil, nil, invoker)
	if !had || time.Until(seen) > 2*time.Second || time.Until(seen) < time.Second {
		t.Errorf("a deadline of about 2s should be added: had=%v remaining=%v", had, time.Until(seen))
	}

	own, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	want, _ := own.Deadline()
	_ = grpcclient.WithDefaultDeadline(2*time.Second)(own, "/x/y", nil, nil, nil, invoker)
	if !seen.Equal(want) {
		t.Errorf("the caller's deadline was replaced: %v vs %v", seen, want)
	}

	_ = grpcclient.WithDefaultDeadline(0)(context.Background(), "/x/y", nil, nil, nil, invoker)
	if had2 := func() bool { _, ok := context.Background().Deadline(); return ok }(); had2 {
		t.Error("unexpected")
	}
}

func TestServiceConfig(t *testing.T) {
	encoded, err := grpcclient.ServiceConfig([]string{
		"/orcan.v1.ProductService/GetProduct",
		"/payment.v1.PaymentService/CreatePayment",
	})
	if err != nil {
		t.Fatal(err)
	}

	var config struct {
		MethodConfig []struct {
			Name []struct {
				Service string `json:"service"`
				Method  string `json:"method"`
			} `json:"name"`
			RetryPolicy struct {
				MaxAttempts          int      `json:"maxAttempts"`
				InitialBackoff       string   `json:"initialBackoff"`
				MaxBackoff           string   `json:"maxBackoff"`
				BackoffMultiplier    float64  `json:"backoffMultiplier"`
				RetryableStatusCodes []string `json:"retryableStatusCodes"`
			} `json:"retryPolicy"`
		} `json:"methodConfig"`
		RetryThrottling struct {
			MaxTokens  int     `json:"maxTokens"`
			TokenRatio float64 `json:"tokenRatio"`
		} `json:"retryThrottling"`
	}
	if err := json.Unmarshal([]byte(encoded), &config); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if len(config.MethodConfig) != 1 || len(config.MethodConfig[0].Name) != 2 {
		t.Fatalf("unexpected method config: %s", encoded)
	}
	names := config.MethodConfig[0].Name
	if names[0].Service != "orcan.v1.ProductService" || names[0].Method != "GetProduct" || names[1].Service != "payment.v1.PaymentService" || names[1].Method != "CreatePayment" {
		t.Errorf("method names were split wrongly: %+v", names)
	}
	policy := config.MethodConfig[0].RetryPolicy
	if policy.MaxAttempts != grpcclient.MaxAttempts || policy.BackoffMultiplier != 2 || policy.InitialBackoff == "" || policy.MaxBackoff == "" {
		t.Errorf("unexpected policy: %+v", policy)
	}
	// 再試行するのは UNAVAILABLE だけ。
	if len(policy.RetryableStatusCodes) != 1 || policy.RetryableStatusCodes[0] != "UNAVAILABLE" {
		t.Errorf("retryable status codes = %v, want only UNAVAILABLE", policy.RetryableStatusCodes)
	}
	// 失敗が続いているときに、再試行が集中しないための、予算。
	if config.RetryThrottling.MaxTokens <= 0 || config.RetryThrottling.TokenRatio <= 0 {
		t.Errorf("retry throttling is missing: %+v", config.RetryThrottling)
	}
}

func TestServiceConfigRejectsInvalidMethodNames(t *testing.T) {
	for _, bad := range []string{"", "GetProduct", "/OnlyService", "/orcan.v1.ProductService/", "//Method"} {
		if _, err := grpcclient.ServiceConfig([]string{bad}); err == nil {
			t.Errorf("ServiceConfig accepted %q", bad)
		}
	}
}

// リクエスト ID は、呼び出し先(orcan-api / payment-api)に渡る。再試行のときも、同じ ID が付く。
// これで、1つのリクエストを、サービスをまたいで、同じ request_id で追える。
func TestTheRequestIDIsForwardedToTheService(t *testing.T) {
	s := newScript()
	s.failFirst("GetProduct", codes.Unavailable, 1)
	c := start(t, s, 5*time.Second)
	ctx := requestid.WithContext(context.Background(), "req-from-http")

	if _, err := c.products.GetProduct(ctx, &orcanv1.GetProductRequest{Id: 1}); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if got := s.requestIDs["GetProduct"]; len(got) != 2 || got[0] != "req-from-http" || got[1] != "req-from-http" {
		t.Errorf("request IDs received per attempt = %v, want [req-from-http req-from-http]", got)
	}
}

// リクエスト ID が無いとき(起動時の呼び出しなど)は、メタデータを付けない。
func TestNoRequestIDMeansNoMetadata(t *testing.T) {
	s := newScript()
	c := start(t, s, 5*time.Second)

	if _, err := c.products.GetProduct(context.Background(), &orcanv1.GetProductRequest{Id: 1}); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if got := s.requestIDs["GetProduct"]; len(got) != 0 {
		t.Errorf("unexpected request IDs: %v", got)
	}
}
