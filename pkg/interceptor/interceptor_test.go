package interceptor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/requestid"
)

func ctxWithToken(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalTokenMetadataKey, token))
}

func okHandler(context.Context, any) (any, error) { return "ok", nil }

func TestAuth(t *testing.T) {
	const secret = "shared-secret"
	tests := []struct {
		name       string
		serverTok  string
		ctx        context.Context
		method     string
		wantCode   codes.Code
		wantCalled bool
	}{
		{"valid token", secret, ctxWithToken(secret), "/orcan.v1.ProductService/ListProducts", codes.OK, true},
		{"wrong token", secret, ctxWithToken("nope"), "/orcan.v1.ProductService/ListProducts", codes.Unauthenticated, false},
		{"no metadata", secret, context.Background(), "/orcan.v1.ProductService/ListProducts", codes.Unauthenticated, false},
		{"no token header", secret, metadata.NewIncomingContext(context.Background(), metadata.MD{}), "/orcan.v1.ProductService/ListProducts", codes.Unauthenticated, false},
		// サーバー側のトークンが未設定のとき、空のトークンで素通りさせてはいけない。
		{"server token not configured", "", ctxWithToken(""), "/orcan.v1.ProductService/ListProducts", codes.Unauthenticated, false},
		// kubeletのヘルスチェックはトークンを持たないので、認証の対象外。
		{"health check needs no token", secret, context.Background(), "/grpc.health.v1.Health/Check", codes.OK, true},
		{"health watch needs no token", secret, context.Background(), "/grpc.health.v1.Health/Watch", codes.OK, true},
		{"a look-alike method is not exempt", secret, context.Background(), "/grpc.health.v1.HealthEvil/Check", codes.Unauthenticated, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := func(ctx context.Context, req any) (any, error) {
				called = true
				return okHandler(ctx, req)
			}
			_, err := Auth(tt.serverTok)(tt.ctx, nil, &grpc.UnaryServerInfo{FullMethod: tt.method}, handler)

			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v (err=%v)", got, tt.wantCode, err)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

// Datadog の標準属性にそろえた、1行分のログ。
type logLine struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Service   string `json:"service"`
	RequestID string `json:"request_id"`
	Duration  *int64 `json:"duration"`
	RPC       struct {
		System  string `json:"system"`
		Service string `json:"service"`
		Method  string `json:"method"`
		GRPC    struct {
			StatusCode string `json:"status_code"`
		} `json:"grpc"`
	} `json:"rpc"`
	Error struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"error"`
}

func runLoggingWith(t *testing.T, ctx context.Context, method string, handlerErr error) (lines []logLine, err error) {
	t.Helper()
	var buffer bytes.Buffer
	logger := logging.New(logging.Config{Service: "orcan-api", Writer: &buffer})
	_, err = Logging(logger)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		return "response", handlerErr
	})
	for _, raw := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		if raw == "" {
			continue
		}
		var line logLine
		if jsonErr := json.Unmarshal([]byte(raw), &line); jsonErr != nil {
			t.Fatalf("log line is not JSON: %q", raw)
		}
		lines = append(lines, line)
	}
	return lines, err
}

func runLogging(t *testing.T, method string, handlerErr error) ([]logLine, error) {
	t.Helper()
	return runLoggingWith(t, context.Background(), method, handlerErr)
}

func TestLoggingLevels(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus string
		wantMsg    string
		wantCode   string
	}{
		{"success", nil, "info", "grpc request completed", "OK"},
		{"client error is a warning", status.Error(codes.InvalidArgument, "bad"), "warn", "grpc request rejected", "InvalidArgument"},
		{"not found is a warning", status.Error(codes.NotFound, "missing"), "warn", "grpc request rejected", "NotFound"},
		{"server error is an error", status.Error(codes.Internal, "internal server error"), "error", "grpc request failed", "Internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := runLogging(t, "/orcan.v1.ProductService/GetProduct", tt.err)

			if !errors.Is(err, tt.err) && err != tt.err {
				t.Errorf("Logging must return the handler's error unchanged: got %v", err)
			}
			if len(lines) != 1 {
				t.Fatalf("expected exactly one log line, got %d", len(lines))
			}
			line := lines[0]
			if line.Status != tt.wantStatus || line.Message != tt.wantMsg {
				t.Errorf("log = {%s %q}, want {%s %q}", line.Status, line.Message, tt.wantStatus, tt.wantMsg)
			}
			if line.RPC.GRPC.StatusCode != tt.wantCode {
				t.Errorf("rpc.grpc.status_code = %q, want %q", line.RPC.GRPC.StatusCode, tt.wantCode)
			}
		})
	}
}

// 属性が、Datadog の標準属性(と、OpenTelemetry の rpc.*)にそろっている。
func TestLoggingUsesStandardAttributes(t *testing.T) {
	ctx := requestid.WithContext(context.Background(), "req-abc")

	lines, _ := runLoggingWith(t, ctx, "/orcan.v1.ProductService/GetProduct", nil)

	line := lines[0]
	if line.Service != "orcan-api" {
		t.Errorf("service = %q", line.Service)
	}
	if line.RPC.System != "grpc" || line.RPC.Service != "orcan.v1.ProductService" || line.RPC.Method != "GetProduct" {
		t.Errorf("rpc = %+v", line.RPC)
	}
	if line.RequestID != "req-abc" {
		t.Errorf("request_id = %q, want req-abc", line.RequestID)
	}
	if line.Duration == nil || *line.Duration < 0 {
		t.Errorf("duration (ns) is missing: %v", line.Duration)
	}
}

// 4xx 相当のエラーでは、error.message に、クライアントに返したメッセージが出る。
func TestLoggingIncludesTheClientErrorMessage(t *testing.T) {
	lines, _ := runLogging(t, "/orcan.v1.ProductService/GetProduct", status.Error(codes.NotFound, "product not found"))

	if got := lines[0].Error; got.Kind != "NotFound" || got.Message != "product not found" {
		t.Errorf("error = %+v", got)
	}
}

// サーバー起因のエラーでは、クライアントには汎用メッセージしか返らないので、本当の原因をログに残す。
func TestLoggingKeepsTheRealCauseForServerErrors(t *testing.T) {
	cause := status.Error(codes.Internal, "database connection reset")
	lines, _ := runLogging(t, "/orcan.v1.ProductService/GetProduct", cause)

	if len(lines) != 1 || !strings.Contains(lines[0].Error.Message, "database connection reset") || lines[0].Error.Kind == "" {
		t.Errorf("the cause was not logged: %+v", lines)
	}
}

// ヘルスチェックは数秒おきに呼ばれるので、ログを埋めないよう出さない。
func TestLoggingSkipsHealthChecks(t *testing.T) {
	lines, err := runLogging(t, "/grpc.health.v1.Health/Check", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Errorf("health checks must not be logged, got %+v", lines)
	}
}

func TestIsHealthCheck(t *testing.T) {
	for method, want := range map[string]bool{
		"/grpc.health.v1.Health/Check":        true,
		"/grpc.health.v1.Health/Watch":        true,
		"/orcan.v1.ProductService/GetProduct": false,
		"/grpc.health.v1.HealthEvil/Check":    false,
		"":                                    false,
	} {
		if got := isHealthCheck(method); got != want {
			t.Errorf("isHealthCheck(%q) = %v, want %v", method, got, want)
		}
	}
}

// リクエスト ID: 呼び出し元から渡されたものを使い、無い・変なときは作る。
func TestRequestIDInterceptor(t *testing.T) {
	var seen string
	handler := func(ctx context.Context, _ any) (any, error) {
		seen = requestid.FromContext(ctx)
		return nil, nil
	}
	call := func(ctx context.Context) string {
		_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x/y"}, handler)
		return seen
	}

	passed := metadata.NewIncomingContext(context.Background(), metadata.Pairs(requestid.MetadataKey, "from-pingu-api"))
	if got := call(passed); got != "from-pingu-api" {
		t.Errorf("the caller's request ID should be used: %q", got)
	}
	if got := call(context.Background()); len(got) != 32 {
		t.Errorf("a missing request ID should be generated: %q", got)
	}
	bad := metadata.NewIncomingContext(context.Background(), metadata.Pairs(requestid.MetadataKey, "bad id\nwith newline"))
	if got := call(bad); got == "bad id\nwith newline" || len(got) != 32 {
		t.Errorf("an unsafe request ID must be replaced: %q", got)
	}
}
