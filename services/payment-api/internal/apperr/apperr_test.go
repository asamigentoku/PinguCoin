package apperr

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func TestConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantCode   codes.Code
		wantReason Reason
		wantMsg    string
	}{
		{"not found", NotFound("payment", nil), codes.NotFound, ReasonNotFound, "payment not found"},
		{"invalid argument", InvalidArgument("amount must be greater than zero"), codes.InvalidArgument, ReasonInvalidArgument, "amount must be greater than zero"},
		{"failed precondition", FailedPrecondition("insufficient points"), codes.FailedPrecondition, ReasonFailedPrecondition, "insufficient points"},
		{"internal", Internal(errors.New("boom")), codes.Internal, ReasonInternal, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.GRPCCode != tt.wantCode || tt.err.Reason != tt.wantReason || tt.err.Message != tt.wantMsg {
				t.Errorf("got {%v %v %q}, want {%v %v %q}", tt.err.GRPCCode, tt.err.Reason, tt.err.Message, tt.wantCode, tt.wantReason, tt.wantMsg)
			}
		})
	}
}

// クライアントに返すステータスには、DBエラーなどの内部情報を含めない(ログ用にだけ保持する)。
func TestInternalDoesNotLeakTheCause(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	appErr := Internal(cause)

	st := status.Convert(appErr)
	if strings.Contains(st.Message(), "10.0.0.5") {
		t.Errorf("client-facing message leaks the cause: %q", st.Message())
	}
	if !strings.Contains(appErr.Error(), "10.0.0.5") {
		t.Errorf("Error() should keep the cause for server logs, got %q", appErr.Error())
	}
	if !errors.Is(appErr, cause) {
		t.Error("errors.Is should reach the wrapped cause")
	}
}

func TestGRPCStatusCarriesErrorInfo(t *testing.T) {
	st := status.Convert(FailedPrecondition("insufficient points"))

	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v", st.Code())
	}
	var info *errdetails.ErrorInfo
	for _, detail := range st.Details() {
		if candidate, ok := detail.(*errdetails.ErrorInfo); ok {
			info = candidate
		}
	}
	if info == nil {
		t.Fatal("no ErrorInfo detail on the status")
	}
	if info.GetReason() != string(ReasonFailedPrecondition) || info.GetDomain() != "payment-api" {
		t.Errorf("ErrorInfo = {reason: %q, domain: %q}", info.GetReason(), info.GetDomain())
	}
}

func TestAs(t *testing.T) {
	notFound := NotFound("payment", gorm.ErrRecordNotFound)

	got, ok := As(fmt.Errorf("wrapped: %w", notFound))
	if !ok || got != notFound {
		t.Fatalf("As() did not find the AppError through a wrapper: %v %v", got, ok)
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Error("As() should be false for a non-AppError")
	}
	if !errors.Is(notFound, gorm.ErrRecordNotFound) {
		t.Error("NotFound should unwrap to the original error")
	}
}
