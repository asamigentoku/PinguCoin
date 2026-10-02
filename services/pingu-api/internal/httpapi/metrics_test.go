package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
)

// metricValue は、登録されているメトリクスから、名前とラベルが合うものの値(カウンター)を読む。見つからなければ 0。
// pkg/metrics の中の変数は、このパッケージからは見えないので、公開されている形(Gather)で読む。
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

func parseOperation(t *testing.T, query string) *ast.OperationDefinition {
	t.Helper()
	document, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		t.Fatal(err)
	}
	return document.Operations[0]
}

func TestRootFields(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"one field", `query { products { id } }`, []string{"products"}},
		// 操作の名前(Anything)は、クライアントが決められるので、ラベルに使わない。ルートのフィールド名だけを使う。
		{"operation name is ignored", `query Anything { products { id } me { id } }`, []string{"products", "me"}},
		{"mutation", `mutation { createProduct(input: {}) { id } }`, []string{"createProduct"}},
		{"introspection is merged", `{ __schema { types { name } } __type(name: "X") { name } __typename }`, []string{"__introspection", "__introspection", "__introspection"}},
		// フラグメントは、ルートのフィールドとしては数えない(ラベルを増やさない)。
		{"fragment spread is skipped", `{ ...F } fragment F on Query { products { id } }`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rootFields(parseOperation(t, tt.query))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("rootFields = %v, want %v", got, tt.want)
			}
		})
	}
}

func newMetricsRouter() http.Handler {
	return NewRouter(slog.New(slog.DiscardHandler), &orcanclient.Client{}, &paymentclient.Client{}, nil, nil)
}

func postGraphQL(router http.Handler, query string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/graphql", strings.NewReader(`{"query":`+jsonString(query)+`}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// GraphQL は、エラーでも HTTP 200 を返すので、HTTP のメトリクスだけでは、失敗が見えない。
// 操作の数を、種類・ルートのフィールド・結果(ok / error)で数える。
func TestGraphQLOperationsAreCounted(t *testing.T) {
	router := newMetricsRouter()
	okBefore := promtestutil.ToFloat64(graphqlOperations.WithLabelValues("query", "__introspection", "ok"))
	invalidBefore := promtestutil.ToFloat64(graphqlOperations.WithLabelValues(invalidOperation, invalidOperation, "error"))

	if recorder := postGraphQL(router, `{ __typename }`); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	// スキーマに無いフィールド = 検証に失敗する(操作として成立しない)。生のフィールド名は、ラベルにならない。
	postGraphQL(router, `{ thisFieldDoesNotExist }`)

	if got := promtestutil.ToFloat64(graphqlOperations.WithLabelValues("query", "__introspection", "ok")) - okBefore; got != 1 {
		t.Errorf("ok operations increased by %v, want 1", got)
	}
	if got := promtestutil.ToFloat64(graphqlOperations.WithLabelValues(invalidOperation, invalidOperation, "error")) - invalidBefore; got != 1 {
		t.Errorf("invalid operations increased by %v, want 1", got)
	}
}

// ルーター全体: リクエストは、実際のパスではなく、ルートのパターンで数えられる。
func TestRouterCountsRequestsByRoutePattern(t *testing.T) {
	router := newMetricsRouter()
	labels := map[string]string{"method": "GET", "route": "/healthz", "code": "200"}
	before := metricValue(t, "pingucoin_http_requests_total", labels)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := metricValue(t, "pingucoin_http_requests_total", labels) - before; got != 1 {
		t.Errorf("/healthz requests increased by %v, want 1", got)
	}
}

// /metrics は、API のルーターには無い(別のポート。インターネットに向けた入口から、見えてはいけない)。
func TestRouterDoesNotServeMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	newMetricsRouter().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if strings.Contains(recorder.Body.String(), "pingucoin_http_requests_total") {
		t.Error("the API router exposes /metrics; it must be served on a separate port")
	}
}

func TestAnonymousRequestsAreCountedAsAnonymous(t *testing.T) {
	labels := map[string]string{"result": "anonymous"}
	before := metricValue(t, "pingucoin_auth_requests_total", labels)

	newMetricsRouter().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := metricValue(t, "pingucoin_auth_requests_total", labels) - before; got != 1 {
		t.Errorf("anonymous count increased by %v, want 1", got)
	}
}

func TestOrderFailureResult(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"conflict is the business reason", status.Error(codes.FailedPrecondition, "insufficient stock"), "out_of_stock"},
		{"not found is the caller's problem", status.Error(codes.NotFound, "no product"), "rejected"},
		{"internal is a real failure", status.Error(codes.Internal, "boom"), "failed"},
		{"unavailable is a real failure", status.Error(codes.Unavailable, "down"), "failed"},
		{"timeout is a real failure", status.Error(codes.DeadlineExceeded, "slow"), "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orderFailureResult(apperr.FromGRPC(tt.err), "out_of_stock"); got != tt.want {
				t.Errorf("orderFailureResult = %q, want %q", got, tt.want)
			}
		})
	}
}
