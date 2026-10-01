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
			writeError(recorder, tt.err)

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
		writeError(recorder, err)

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

func TestWithLogging(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /bad", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) })
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := WithLogging(logger)(mux)

	levelOf := func(path string) string {
		buffer.Reset()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
		if buffer.Len() == 0 {
			return ""
		}
		var line struct {
			Level  string `json:"level"`
			Status int    `json:"status"`
			Path   string `json:"path"`
		}
		if err := json.Unmarshal(buffer.Bytes(), &line); err != nil {
			t.Fatalf("log line is not JSON: %s", buffer.String())
		}
		if line.Path != path {
			t.Errorf("logged path = %q, want %q", line.Path, path)
		}
		return line.Level
	}

	for path, want := range map[string]string{"/ok": "INFO", "/bad": "WARN", "/boom": "ERROR"} {
		if got := levelOf(path); got != want {
			t.Errorf("%s logged at %q, want %q", path, got, want)
		}
	}
	// ヘルスチェックは数秒おきに呼ばれるので、ログを埋めないよう出さない。
	for _, path := range []string{"/healthz", "/readyz"} {
		if got := levelOf(path); got != "" {
			t.Errorf("%s should not be logged, got a %s line", path, got)
		}
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
