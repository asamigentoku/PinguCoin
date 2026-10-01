package interceptor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

type logLine struct {
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Method  string `json:"method"`
	Code    string `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func runLogging(t *testing.T, method string, handlerErr error) (lines []logLine, err error) {
	t.Helper()
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	_, err = Logging(logger)(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
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

func TestLoggingLevels(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantLevel string
		wantMsg   string
	}{
		{"success", nil, "INFO", "grpc request completed"},
		{"client error is a warning", status.Error(codes.InvalidArgument, "bad"), "WARN", "grpc request rejected"},
		{"not found is a warning", status.Error(codes.NotFound, "missing"), "WARN", "grpc request rejected"},
		{"server error is an error", status.Error(codes.Internal, "internal server error"), "ERROR", "grpc request failed"},
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
			if lines[0].Level != tt.wantLevel || lines[0].Msg != tt.wantMsg {
				t.Errorf("log = {%s %q}, want {%s %q}", lines[0].Level, lines[0].Msg, tt.wantLevel, tt.wantMsg)
			}
			if lines[0].Method != "/orcan.v1.ProductService/GetProduct" {
				t.Errorf("method = %q", lines[0].Method)
			}
		})
	}
}

// サーバー起因のエラーでは、クライアントには汎用メッセージしか返らないので、本当の原因をログに残す。
func TestLoggingKeepsTheRealCauseForServerErrors(t *testing.T) {
	cause := status.Error(codes.Internal, "pq: deadlock detected")
	lines, _ := runLogging(t, "/orcan.v1.ProductService/GetProduct", cause)

	if len(lines) != 1 || !strings.Contains(lines[0].Error, "deadlock detected") {
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
