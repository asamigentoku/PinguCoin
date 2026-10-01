package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/requestid"
	paymentpb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/reqcontext"
)

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("response is not the expected JSON: %v\n%s", err, recorder.Body.String())
	}
	return value
}

func TestWriteErrorMapsAppErrorsToHTTP(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantReason apperr.Reason
	}{
		{"unauthenticated", apperr.Unauthenticated("login is required"), http.StatusUnauthorized, apperr.ReasonUnauthenticated},
		{"invalid argument", apperr.InvalidArgument("bad"), http.StatusBadRequest, apperr.ReasonInvalidArgument},
		{"permission denied", apperr.PermissionDenied("no"), http.StatusForbidden, apperr.ReasonPermissionDenied},
		{"failed precondition", apperr.FailedPrecondition("insufficient points"), http.StatusConflict, apperr.ReasonFailedPrecondition},
		{"not found", apperr.NotFound("order"), http.StatusNotFound, apperr.ReasonNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeError(recorder, httptest.NewRequest("GET", "/", nil), tt.err)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			body := decode[errorResponse](t, recorder)
			if body.Reason != string(tt.wantReason) {
				t.Errorf("reason = %q, want %q", body.Reason, tt.wantReason)
			}
			if ct := recorder.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
		})
	}
}

// 想定外のエラー(DBエラーなど)の詳細は、クライアントに返さない。
func TestWriteErrorHidesInternalDetails(t *testing.T) {
	for _, err := range []error{errors.New("dial tcp 10.0.0.5:5432: refused"), apperr.Internal(errors.New("dial tcp 10.0.0.5:5432: refused"))} {
		recorder := httptest.NewRecorder()
		writeError(recorder, httptest.NewRequest("GET", "/", nil), err)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "10.0.0.5") {
			t.Errorf("response leaks the cause: %s", recorder.Body.String())
		}
	}
}

func TestHealthEndpoints(t *testing.T) {
	var pingErr error
	handler := NewHealthHandler(func(context.Context) error { return pingErr })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.Live)
	mux.HandleFunc("GET /readyz", handler.Ready)

	if got := call(mux, "GET", "/healthz", callOptions{}); got.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", got.Code)
	}
	if got := call(mux, "GET", "/readyz", callOptions{}); got.Code != http.StatusOK {
		t.Errorf("/readyz = %d, want 200 while the database is reachable", got.Code)
	}

	// DBが落ちたら readiness だけが 503 になる。liveness は 200 のまま(DBの障害でPodを再起動しない)。
	pingErr = errors.New("connection refused")
	ready := call(mux, "GET", "/readyz", callOptions{})
	if ready.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz = %d, want 503 during a database outage", ready.Code)
	}
	if strings.Contains(ready.Body.String(), "connection refused") {
		t.Errorf("/readyz leaks the cause: %s", ready.Body.String())
	}
	if got := call(mux, "GET", "/healthz", callOptions{}); got.Code != http.StatusOK {
		t.Errorf("/healthz = %d during a database outage, want 200", got.Code)
	}
}

// ログの1行(Datadog の標準属性)。
type httpLogLine struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Duration  *int64 `json:"duration"`
	HTTP      struct {
		Method     string `json:"method"`
		URL        string `json:"url"`
		StatusCode int    `json:"status_code"`
		UserAgent  string `json:"useragent"`
		Referer    string `json:"referer"`
	} `json:"http"`
	Network struct {
		Client struct {
			IP string `json:"ip"`
		} `json:"client"`
	} `json:"network"`
}

func serveAndLog(t *testing.T, handler http.Handler, request *http.Request, buffer *bytes.Buffer) (line httpLogLine, logged bool) {
	t.Helper()
	buffer.Reset()
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if buffer.Len() == 0 {
		return line, false
	}
	if err := json.Unmarshal(buffer.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %s", buffer.String())
	}
	return line, true
}

func TestWithLogging(t *testing.T) {
	var buffer bytes.Buffer
	logger := logging.New(logging.Config{Service: "pingu-api", Writer: &buffer})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /bad", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) })
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := WithLogging(logger)(mux)

	for path, want := range map[string]struct {
		status string
		code   int
	}{"/ok": {"info", 200}, "/bad": {"warn", 400}, "/boom": {"error", 500}} {
		line, logged := serveAndLog(t, handler, httptest.NewRequest("GET", path, nil), &buffer)
		if !logged || line.Status != want.status || line.HTTP.StatusCode != want.code || line.HTTP.URL != path || line.HTTP.Method != "GET" {
			t.Errorf("%s: logged=%v line=%+v, want status %q code %d", path, logged, line, want.status, want.code)
		}
		if line.Duration == nil || *line.Duration < 0 {
			t.Errorf("%s: duration (ns) is missing", path)
		}
	}
	// ヘルスチェックは数秒おきに呼ばれるので、ログを埋めないよう出さない。
	for _, path := range []string{"/healthz", "/readyz"} {
		if _, logged := serveAndLog(t, handler, httptest.NewRequest("GET", path, nil), &buffer); logged {
			t.Errorf("%s should not be logged", path)
		}
	}
}

// クエリ文字列(トークンなどの秘密を含みうる)は、ログに出さない。User-Agent / Referer は、標準属性として出す。
func TestWithLoggingStandardAttributesAndNoQueryString(t *testing.T) {
	var buffer bytes.Buffer
	handler := WithLogging(logging.New(logging.Config{Service: "pingu-api", Writer: &buffer}))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	request := httptest.NewRequest("GET", "/api/v1/orders?token=SECRET&x=1", nil)
	request.Header.Set("User-Agent", "test-agent/1.0")
	request.Header.Set("Referer", "https://pingucoin.example/cart")

	line, _ := serveAndLog(t, handler, request, &buffer)

	if line.HTTP.URL != "/api/v1/orders" || strings.Contains(buffer.String(), "SECRET") {
		t.Errorf("the query string leaked into the log: %s", buffer.String())
	}
	if line.HTTP.UserAgent != "test-agent/1.0" || line.HTTP.Referer != "https://pingucoin.example/cart" {
		t.Errorf("http.useragent / http.referer = %q / %q", line.HTTP.UserAgent, line.HTTP.Referer)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"direct connection", "203.0.113.7:51234", "", "203.0.113.7"},
		{"behind the ingress: the original client", "10.244.0.5:40000", "198.51.100.9, 10.244.0.5", "198.51.100.9"},
		{"a malformed forwarded header is ignored", "203.0.113.7:51234", "not-an-ip", "203.0.113.7"},
		{"an address without a port", "203.0.113.7", "", "203.0.113.7"},
	}
	for _, tt := range tests {
		request := httptest.NewRequest("GET", "/", nil)
		request.RemoteAddr = tt.remoteAddr
		if tt.forwarded != "" {
			request.Header.Set("X-Forwarded-For", tt.forwarded)
		}
		if got := clientIP(request); got != tt.want {
			t.Errorf("%s: clientIP = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// リクエスト ID: 来たものを使い(安全な文字だけ)、無ければ作る。レスポンスにも、ログにも、同じ ID が出る。
func TestRequestIDMiddleware(t *testing.T) {
	var seen string
	var buffer bytes.Buffer
	logger := logging.New(logging.Config{Service: "pingu-api", Writer: &buffer})
	handler := WithRequestID(WithLogging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestid.FromContext(r.Context())
	})))

	// 来た ID は、そのまま使う。
	request := httptest.NewRequest("GET", "/x", nil)
	request.Header.Set(requestid.Header, "from-nextjs-123")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if seen != "from-nextjs-123" || recorder.Header().Get(requestid.Header) != "from-nextjs-123" {
		t.Errorf("seen=%q response=%q", seen, recorder.Header().Get(requestid.Header))
	}
	var line httpLogLine
	if err := json.Unmarshal(buffer.Bytes(), &line); err != nil || line.RequestID != "from-nextjs-123" {
		t.Errorf("the log line must carry the same request_id: %+v (%v)", line, err)
	}

	// 無いときは、作る。
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/x", nil))
	if len(seen) != 32 || recorder.Header().Get(requestid.Header) != seen {
		t.Errorf("a missing ID should be generated and echoed: seen=%q header=%q", seen, recorder.Header().Get(requestid.Header))
	}

	// 変な ID(改行・長すぎ)は、使わない(ログの偽装を防ぐ)。
	request = httptest.NewRequest("GET", "/x", nil)
	request.Header.Set(requestid.Header, strings.Repeat("a", 300))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if len(seen) != 32 {
		t.Errorf("an unsafe request ID must be replaced: %q", seen)
	}
}

func TestAPIVersionHeader(t *testing.T) {
	handler := WithAPIVersion(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	for path, want := range map[string]string{"/api/v1/orders": "v1", "/api/v1/graphql": "v1", "/healthz": "", "/version": "", "/": ""} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if got := recorder.Header().Get("X-API-Version"); got != want {
			t.Errorf("%s: X-API-Version = %q, want %q", path, got, want)
		}
	}
}

func TestVersionEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", NewHealthHandler(nil).Version)

	recorder := call(mux, "GET", "/version", callOptions{})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := decode[map[string]string](t, recorder)
	for _, key := range []string{"service", "version", "commit", "build_time", "go_version", "api"} {
		if body[key] == "" {
			t.Errorf("%q is missing from %v", key, body)
		}
	}
	if body["service"] != "pingu-api" || body["api"] != APIVersion {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := map[string]string{
		"Bearer abc.def.ghi": "abc.def.ghi",
		"Bearer   spaced  ":  "spaced",
		"bearer lowercase":   "",
		"Basic dXNlcjpwYXNz": "",
		"Bearer":             "",
		"":                   "",
	}
	for header, want := range tests {
		request := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			request.Header.Set("Authorization", header)
		}
		if got := extractBearerToken(request); got != want {
			t.Errorf("extractBearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

// トークンが無い・不正でもリクエストは通る(公開クエリのため)。ただしユーザーは設定されない。
func TestWithOptionalAuthLetsAnonymousRequestsThrough(t *testing.T) {
	reached := false
	handler := WithOptionalAuth(&orcanclient.Client{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if _, ok := reqcontext.UserFromContext(r.Context()); ok {
			t.Error("an anonymous request must not carry a user")
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !reached {
		t.Error("an anonymous request was blocked")
	}

	reached = false
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer not-a-real-token")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if !reached {
		t.Error("a request with an invalid token was blocked (it should pass through unauthenticated)")
	}
}

func TestOrderStatusFromPayment(t *testing.T) {
	for paymentStatus, want := range map[string]string{
		"succeeded": "paid", "pending": "pending", "failed": "failed", "canceled": "failed", "": "failed", "refunded": "failed",
	} {
		if got := orderStatusFromPayment(paymentStatus); got != want {
			t.Errorf("orderStatusFromPayment(%q) = %q, want %q", paymentStatus, got, want)
		}
	}
}

func TestOrdersRequireLogin(t *testing.T) {
	handler := NewOrderHandler(slog.Default(), &orcanclient.Client{}, &paymentclient.Client{}, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/orders", handler.CreateOrder)
	mux.HandleFunc("GET /api/v1/orders", handler.ListOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", handler.GetOrder)
	router := asUser(mux)

	for _, endpoint := range [][2]string{{"POST", "/api/v1/orders"}, {"GET", "/api/v1/orders"}, {"GET", "/api/v1/orders/1"}} {
		recorder := call(router, endpoint[0], endpoint[1], callOptions{key: "k", body: `{"product_id":1}`})
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", endpoint[0], endpoint[1], recorder.Code)
		}
	}
}

func TestCreateOrderRequiresAnIdempotencyKey(t *testing.T) {
	handler := NewOrderHandler(slog.Default(), &orcanclient.Client{}, &paymentclient.Client{}, nil)
	router := asUser(http.HandlerFunc(handler.CreateOrder))

	recorder := call(router, "POST", "/api/v1/orders", callOptions{user: "1", body: `{"product_id":1}`})
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
}

func TestPointsRequireLogin(t *testing.T) {
	handler := NewPointHandler(&paymentclient.Client{Point: newFakePoints()})
	recorder := call(asUser(http.HandlerFunc(handler.GetPoints)), "GET", "/api/v1/points", callOptions{})
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", recorder.Code)
	}
}

type pointsBody struct {
	Balance      int64 `json:"balance"`
	Transactions []struct {
		ID           uint32 `json:"id"`
		Amount       int64  `json:"amount"`
		Type         string `json:"type"`
		Reason       string `json:"reason"`
		BalanceAfter int64  `json:"balance_after"`
	} `json:"transactions"`
}

// 初めての人には、ウェルカムボーナスを1回だけ付与する。
func TestFirstVisitGrantsTheWelcomeBonusOnce(t *testing.T) {
	points := newFakePoints()
	router := asUser(http.HandlerFunc(NewPointHandler(&paymentclient.Client{Point: points}).GetPoints))

	first := call(router, "GET", "/api/v1/points", callOptions{user: "7"})
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", first.Code, first.Body.String())
	}
	body := decode[pointsBody](t, first)
	if body.Balance != welcomeBonusPoints || len(body.Transactions) != 1 || body.Transactions[0].Amount != welcomeBonusPoints {
		t.Errorf("unexpected first response: %+v", body)
	}

	// 2回目以降は付与しない。
	call(router, "GET", "/api/v1/points", callOptions{user: "7"})
	call(router, "GET", "/api/v1/points", callOptions{user: "7"})
	if len(points.credits) != 1 {
		t.Errorf("the bonus was granted %d times, want 1", len(points.credits))
	}
	if points.balance[7] != welcomeBonusPoints {
		t.Errorf("balance = %d, want %d", points.balance[7], welcomeBonusPoints)
	}
}

// ボーナスは、ログイン中のユーザー本人にだけ付く(他のユーザーには影響しない)。
func TestWelcomeBonusIsPerUser(t *testing.T) {
	points := newFakePoints()
	router := asUser(http.HandlerFunc(NewPointHandler(&paymentclient.Client{Point: points}).GetPoints))

	call(router, "GET", "/api/v1/points", callOptions{user: "1"})
	call(router, "GET", "/api/v1/points", callOptions{user: "2"})

	if len(points.credits) != 2 || points.credits[0].GetUserId() != 1 || points.credits[1].GetUserId() != 2 {
		t.Errorf("unexpected credits: %+v", points.credits)
	}
}

// 履歴は、新しい順に返す。
func TestPointHistoryIsNewestFirst(t *testing.T) {
	points := newFakePoints()
	// 既存の履歴(古い順): ボーナス → 購入 → 返金
	for _, amount := range []int64{1600, -300, 300} {
		points.balance[5] += amount
		points.transactions[5] = append(points.transactions[5], &paymentpb.PointTransaction{
			Id: uint32(len(points.transactions[5]) + 1), UserId: 5, Amount: amount, BalanceAfter: points.balance[5],
		})
	}
	router := asUser(http.HandlerFunc(NewPointHandler(&paymentclient.Client{Point: points}).GetPoints))

	body := decode[pointsBody](t, call(router, "GET", "/api/v1/points", callOptions{user: "5"}))

	if len(body.Transactions) != 3 {
		t.Fatalf("got %d transactions, want 3", len(body.Transactions))
	}
	if body.Transactions[0].ID != 3 || body.Transactions[2].ID != 1 {
		t.Errorf("history is not newest-first: ids = %d, %d, %d", body.Transactions[0].ID, body.Transactions[1].ID, body.Transactions[2].ID)
	}
	if body.Balance != 1600 || len(points.credits) != 0 {
		t.Errorf("balance = %d, bonus credits = %d; a user with history must not get the bonus again", body.Balance, len(points.credits))
	}
}
