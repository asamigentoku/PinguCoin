package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantStatus int
		wantReason Reason
	}{
		{"not found", NotFound("order"), http.StatusNotFound, ReasonNotFound},
		{"invalid argument", InvalidArgument("bad"), http.StatusBadRequest, ReasonInvalidArgument},
		{"unauthenticated", Unauthenticated("login is required"), http.StatusUnauthorized, ReasonUnauthenticated},
		{"permission denied", PermissionDenied("no"), http.StatusForbidden, ReasonPermissionDenied},
		{"failed precondition", FailedPrecondition("insufficient points"), http.StatusConflict, ReasonFailedPrecondition},
		{"internal", Internal(errors.New("boom")), http.StatusInternalServerError, ReasonInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.HTTPStatus != tt.wantStatus || tt.err.Reason != tt.wantReason {
				t.Errorf("got {%d %v}, want {%d %v}", tt.err.HTTPStatus, tt.err.Reason, tt.wantStatus, tt.wantReason)
			}
		})
	}
	if got := NotFound("order").Message; got != "order not found" {
		t.Errorf("NotFound message = %q", got)
	}
}

func TestInternalDoesNotLeakTheCause(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	appErr := Internal(cause)

	if strings.Contains(appErr.Message, "10.0.0.5") || appErr.Message != "internal server error" {
		t.Errorf("client-facing message = %q", appErr.Message)
	}
	if !strings.Contains(appErr.Error(), "10.0.0.5") || !errors.Is(appErr, cause) {
		t.Error("the cause must stay reachable for server logs")
	}
}

func TestAs(t *testing.T) {
	notFound := NotFound("order")
	if got, ok := As(fmt.Errorf("wrapped: %w", notFound)); !ok || got != notFound {
		t.Errorf("As() did not see through the wrapper: %v %v", got, ok)
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Error("As() should be false for a non-AppError")
	}
}

// orcan-api / payment-api(gRPC)のエラーを、HTTPのステータスに変換する。
func TestFromGRPC(t *testing.T) {
	tests := []struct {
		code       codes.Code
		wantStatus int
	}{
		{codes.NotFound, http.StatusNotFound},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.FailedPrecondition, http.StatusConflict},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unavailable, http.StatusInternalServerError},
		{codes.Unknown, http.StatusInternalServerError},
		{codes.DeadlineExceeded, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			got := FromGRPC(status.Error(tt.code, "upstream message"))
			if got.HTTPStatus != tt.wantStatus {
				t.Errorf("HTTPStatus = %d, want %d", got.HTTPStatus, tt.wantStatus)
			}
		})
	}
}

// 上流のメッセージは、クライアントに返してよいもの(4xx系)だけ引き継ぐ。それ以外は内部エラーとして隠す。
func TestFromGRPCKeepsClientMessagesButHidesServerErrors(t *testing.T) {
	if got := FromGRPC(status.Error(codes.FailedPrecondition, "insufficient points")); got.Message != "insufficient points" {
		t.Errorf("message = %q, want it passed through", got.Message)
	}

	hidden := FromGRPC(status.Error(codes.Internal, "pq: relation \"payments\" does not exist"))
	if hidden.Message != "internal server error" || strings.Contains(hidden.Message, "payments") {
		t.Errorf("a server error leaked its message: %q", hidden.Message)
	}
	if hidden.Err == nil {
		t.Error("the original error should be kept for the server log")
	}
}

func TestFromGRPCUsesTheReasonFromErrorInfo(t *testing.T) {
	withDetails, err := status.New(codes.FailedPrecondition, "insufficient points").WithDetails(&errdetails.ErrorInfo{Reason: "INSUFFICIENT_POINTS", Domain: "payment-api"})
	if err != nil {
		t.Fatal(err)
	}

	if got := FromGRPC(withDetails.Err()); got.Reason != "INSUFFICIENT_POINTS" {
		t.Errorf("reason = %q, want the one from ErrorInfo", got.Reason)
	}
	// ErrorInfoが無いときは、gRPCのコード名を理由にする。
	if got := FromGRPC(status.Error(codes.NotFound, "x")); got.Reason != "NotFound" {
		t.Errorf("reason = %q, want the gRPC code name", got.Reason)
	}
}

func TestFromGRPCEdgeCases(t *testing.T) {
	if FromGRPC(nil) != nil {
		t.Error("FromGRPC(nil) should be nil")
	}
	if got := FromGRPC(errors.New("not a gRPC error")); got.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("a non-gRPC error should map to 500, got %d", got.HTTPStatus)
	}
}
