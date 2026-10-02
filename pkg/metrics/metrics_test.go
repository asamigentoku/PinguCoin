package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// 全テストで、同じメトリクス(プロセスで1つ)を使うので、テストごとに、固有のラベルの値を使い、「増えた分」を比べる。

func counter(vec *prometheus.CounterVec, labels ...string) float64 {
	return promtestutil.ToFloat64(vec.WithLabelValues(labels...))
}

func TestSplitMethod(t *testing.T) {
	tests := []struct{ in, wantService, wantMethod string }{
		{"/orcan.v1.ProductService/GetProduct", "orcan.v1.ProductService", "GetProduct"},
		{"orcan.v1.ProductService/GetProduct", "orcan.v1.ProductService", "GetProduct"},
		// 想定外の形は、ラベルを増やさないよう、unknown にまとめる。
		{"", "unknown", "unknown"},
		{"/", "unknown", "unknown"},
		{"/only-service", "unknown", "unknown"},
		{"//Method", "unknown", "unknown"},
	}
	for _, tt := range tests {
		service, method := splitMethod(tt.in)
		if service != tt.wantService || method != tt.wantMethod {
			t.Errorf("splitMethod(%q) = (%q, %q), want (%q, %q)", tt.in, service, method, tt.wantService, tt.wantMethod)
		}
	}
}

func TestUnaryServerInterceptor_CountsByCode(t *testing.T) {
	const method = "/test.v1.ServerCount/Do"
	interceptor := UnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: method}

	run := func(err error) {
		_, _ = interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) { return nil, err })
	}
	okBefore := counter(grpcServerHandled, "test.v1.ServerCount", "Do", "OK")
	notFoundBefore := counter(grpcServerHandled, "test.v1.ServerCount", "Do", "NotFound")

	run(nil)
	run(nil)
	run(status.Error(codes.NotFound, "x"))

	if got := counter(grpcServerHandled, "test.v1.ServerCount", "Do", "OK") - okBefore; got != 2 {
		t.Errorf("OK count increased by %v, want 2", got)
	}
	if got := counter(grpcServerHandled, "test.v1.ServerCount", "Do", "NotFound") - notFoundBefore; got != 1 {
		t.Errorf("NotFound count increased by %v, want 1", got)
	}
}

func TestUnaryServerInterceptor_InFlightReturnsToZero(t *testing.T) {
	before := promtestutil.ToFloat64(grpcServerInFlight)
	var during float64
	_, _ = UnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.v1.InFlight/Do"},
		func(context.Context, any) (any, error) {
			during = promtestutil.ToFloat64(grpcServerInFlight)
			return nil, nil
		})
	if during != before+1 {
		t.Errorf("in-flight during the call = %v, want %v", during, before+1)
	}
	if after := promtestutil.ToFloat64(grpcServerInFlight); after != before {
		t.Errorf("in-flight after the call = %v, want %v", after, before)
	}
}

func TestUnaryServerInterceptor_IgnoresHealthChecks(t *testing.T) {
	before := counter(grpcServerHandled, "grpc.health.v1.Health", "Check", "OK")
	_, _ = UnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(context.Context, any) (any, error) { return nil, nil })
	if got := counter(grpcServerHandled, "grpc.health.v1.Health", "Check", "OK"); got != before {
		t.Errorf("health check was counted (%v -> %v); kubelet probes must not be counted", before, got)
	}
}

func TestUnaryClientInterceptor_CountsByCode(t *testing.T) {
	const target = "test-target"
	invoker := func(err error) grpc.UnaryInvoker {
		return func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error { return err }
	}
	interceptor := UnaryClientInterceptor(target)
	unavailableBefore := counter(grpcClientHandled, target, "test.v1.Client", "Do", "Unavailable")

	_ = interceptor(context.Background(), "/test.v1.Client/Do", nil, nil, nil, invoker(status.Error(codes.Unavailable, "down")))

	if got := counter(grpcClientHandled, target, "test.v1.Client", "Do", "Unavailable") - unavailableBefore; got != 1 {
		t.Errorf("Unavailable count increased by %v, want 1", got)
	}
}

// 再試行を含めた「実際に送った回数」を数える(handled_total との差が、再試行の数)。
func TestClientStatsHandler_CountsEveryAttempt(t *testing.T) {
	const target = "test-attempts"
	handler := ClientStatsHandler(target)
	before := counter(grpcClientAttempts, target, "test.v1.Attempts", "Do")

	ctx := handler.TagRPC(context.Background(), &stats.RPCTagInfo{FullMethodName: "/test.v1.Attempts/Do"})
	handler.HandleRPC(ctx, &stats.Begin{})                                // 1回目
	handler.HandleRPC(ctx, &stats.OutHeader{})                            // Begin 以外のイベントは、数えない
	handler.HandleRPC(ctx, &stats.End{})                                  // 同上
	handler.HandleRPC(ctx, &stats.Begin{IsTransparentRetryAttempt: true}) // 再試行

	if got := counter(grpcClientAttempts, target, "test.v1.Attempts", "Do") - before; got != 2 {
		t.Errorf("attempts increased by %v, want 2", got)
	}
}

func TestRouteFromPattern(t *testing.T) {
	tests := []struct{ pattern, want string }{
		{"POST /api/v1/orders", "/api/v1/orders"},
		{"GET /api/v1/orders/{id}", "/api/v1/orders/{id}"},
		{"/api/v1/graphql", "/api/v1/graphql"},
		{"/", "/"},
		{"", unmatchedRoute},
	}
	for _, tt := range tests {
		if got := routeFromPattern(tt.pattern); got != tt.want {
			t.Errorf("routeFromPattern(%q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

func newTestHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) })
	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	return HTTP(Route(mux))
}

func serve(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

// ラベルには、実際のパス(/items/42)ではなく、パターン(/items/{id})を使う。
// 実際のパスを使うと、ID の数だけ時系列が増える。
func TestHTTP_UsesRoutePatternNotRawPath(t *testing.T) {
	handler := newTestHandler()
	before := counter(httpRequests, "GET", "/items/{id}", "200")
	bytesBefore := counter(httpResponseBytes, "/items/{id}")

	for _, id := range []string{"1", "2", "3"} {
		serve(handler, http.MethodGet, "/items/"+id)
	}

	if got := counter(httpRequests, "GET", "/items/{id}", "200") - before; got != 3 {
		t.Errorf("requests for the pattern increased by %v, want 3", got)
	}
	if got := counter(httpResponseBytes, "/items/{id}") - bytesBefore; got != 15 { // "hello" x 3
		t.Errorf("response bytes increased by %v, want 15", got)
	}
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "pingucoin_http_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "route" && strings.HasPrefix(label.GetValue(), "/items/") && label.GetValue() != "/items/{id}" {
					t.Errorf("a raw path leaked into the route label: %q", label.GetValue())
				}
			}
		}
	}
}

func TestHTTP_RecordsStatusAndUnmatched(t *testing.T) {
	handler := newTestHandler()
	createdBefore := counter(httpRequests, "POST", "/items", "201")
	unmatchedBefore := counter(httpRequests, "GET", unmatchedRoute, "404")

	serve(handler, http.MethodPost, "/items")
	// 存在しない URL を、大量に叩かれても、ラベルは unmatched の 1 つだけ。
	serve(handler, http.MethodGet, "/no/such/path/1")
	serve(handler, http.MethodGet, "/no/such/path/2")

	if got := counter(httpRequests, "POST", "/items", "201") - createdBefore; got != 1 {
		t.Errorf("201 count increased by %v, want 1", got)
	}
	if got := counter(httpRequests, "GET", unmatchedRoute, "404") - unmatchedBefore; got != 2 {
		t.Errorf("unmatched 404 count increased by %v, want 2", got)
	}
}

func TestHTTP_InFlightReturnsToZero(t *testing.T) {
	before := promtestutil.ToFloat64(httpInFlight)
	serve(newTestHandler(), http.MethodGet, "/items/1")
	if after := promtestutil.ToFloat64(httpInFlight); after != before {
		t.Errorf("in-flight after the request = %v, want %v", after, before)
	}
}

// ミドルウェアが、ステータスコードを書かずに、本文だけ書いたときは、200 として数える。
func TestHTTP_ImplicitOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /implicit", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })
	before := counter(httpRequests, "GET", "/implicit", "200")
	serve(HTTP(Route(mux)), http.MethodGet, "/implicit")
	if got := counter(httpRequests, "GET", "/implicit", "200") - before; got != 1 {
		t.Errorf("200 count increased by %v, want 1", got)
	}
}

func TestHandler_ExposesMetricsInPrometheusFormat(t *testing.T) {
	Init("test-service")
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := recorder.Body.String()
	for _, want := range []string{
		"pingucoin_build_info{",
		`service="test-service"`,
		"go_goroutines", // Go のランタイム
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics does not contain %q", want)
		}
	}
}

func TestAddr(t *testing.T) {
	t.Setenv("METRICS_PORT", "")
	if got := Addr(); got != ":9090" {
		t.Errorf("default Addr() = %q, want :9090", got)
	}
	t.Setenv("METRICS_PORT", "9191")
	if got := Addr(); got != ":9191" {
		t.Errorf("Addr() = %q, want :9191", got)
	}
}

func TestResultOf(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ResultSucceeded},
		{status.Error(codes.InvalidArgument, ""), ResultRejected},
		{status.Error(codes.NotFound, ""), ResultRejected},
		{status.Error(codes.FailedPrecondition, ""), ResultRejected},
		{status.Error(codes.Unauthenticated, ""), ResultRejected},
		{status.Error(codes.Internal, ""), ResultFailed},
		{status.Error(codes.Unavailable, ""), ResultFailed},
		{status.Error(codes.DeadlineExceeded, ""), ResultFailed},
		{errors.New("plain error"), ResultFailed}, // gRPC のエラーでないものは Unknown = サーバーの失敗
	}
	for _, tt := range tests {
		if got := ResultOf(tt.err); got != tt.want {
			t.Errorf("ResultOf(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestLabel(t *testing.T) {
	if got := Label("point", "point"); got != "point" {
		t.Errorf("Label(point) = %q", got)
	}
	// 許可していない値は、どんな文字列でも、other にまとまる(時系列を、増やされない)。
	for _, value := range []string{"", "credit_card", "<script>", strings.Repeat("a", 1000)} {
		if got := Label(value, "point"); got != "other" {
			t.Errorf("Label(%q) = %q, want other", value, got)
		}
	}
}

func TestRecordOrder(t *testing.T) {
	createdBefore := counter(orders, OrderCreated)
	amountBefore := promtestutil.ToFloat64(orderAmount)

	RecordOrder(OrderCreated, 300)
	RecordOrder(OrderOutOfStock, 999) // 成立していない注文の金額は、足さない

	if got := counter(orders, OrderCreated) - createdBefore; got != 1 {
		t.Errorf("created orders increased by %v, want 1", got)
	}
	if got := promtestutil.ToFloat64(orderAmount) - amountBefore; got != 300 {
		t.Errorf("order amount increased by %v, want 300 (failed orders must not add)", got)
	}
}

func TestRecordPayment_AddsAmountOnlyOnSuccess(t *testing.T) {
	before := counter(paymentAmount, "POINT")
	RecordPayment("point", "POINT", ResultSucceeded, 500)
	RecordPayment("point", "POINT", ResultInsufficient, 700)
	RecordPayment("point", "POINT", ResultReplayed, 500) // 再送は、二重に足さない
	if got := counter(paymentAmount, "POINT") - before; got != 500 {
		t.Errorf("payment amount increased by %v, want 500", got)
	}
}

func TestRecordInventoryAdjustment_Direction(t *testing.T) {
	incBefore := counter(inventoryAdjustments, "increase", ResultSucceeded)
	decBefore := counter(inventoryAdjustments, "decrease", ResultInsufficient)
	RecordInventoryAdjustment(5, ResultSucceeded)
	RecordInventoryAdjustment(-1, ResultInsufficient)
	if got := counter(inventoryAdjustments, "increase", ResultSucceeded) - incBefore; got != 1 {
		t.Errorf("increase = %v, want 1", got)
	}
	if got := counter(inventoryAdjustments, "decrease", ResultInsufficient) - decBefore; got != 1 {
		t.Errorf("decrease = %v, want 1", got)
	}
}

func TestRecordPointTransaction_AddsAmountOnlyOnSuccess(t *testing.T) {
	before := counter(pointAmount, "credit")
	RecordPointTransaction("credit", ResultSucceeded, 100)
	RecordPointTransaction("credit", ResultFailed, 100)
	if got := counter(pointAmount, "credit") - before; got != 100 {
		t.Errorf("point amount increased by %v, want 100", got)
	}
}

func TestObserveBlob(t *testing.T) {
	okBefore := counter(blobOperations, "test_op", ResultSucceeded)
	failBefore := counter(blobOperations, "test_op", ResultFailed)
	ObserveBlob("test_op", time.Now(), nil)
	ObserveBlob("test_op", time.Now(), errors.New("boom"))
	if got := counter(blobOperations, "test_op", ResultSucceeded) - okBefore; got != 1 {
		t.Errorf("succeeded = %v, want 1", got)
	}
	if got := counter(blobOperations, "test_op", ResultFailed) - failBefore; got != 1 {
		t.Errorf("failed = %v, want 1", got)
	}
}
