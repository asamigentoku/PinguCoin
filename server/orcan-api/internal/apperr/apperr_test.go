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
		{"not found", NotFound("product", nil), codes.NotFound, ReasonNotFound, "product not found"},
		{"invalid argument", InvalidArgument("name is required"), codes.InvalidArgument, ReasonInvalidArgument, "name is required"},
		{"permission denied", PermissionDenied("not yours"), codes.PermissionDenied, ReasonPermissionDenied, "not yours"},
		{"failed precondition", FailedPrecondition("insufficient stock"), codes.FailedPrecondition, ReasonFailedPrecondition, "insufficient stock"},
		{"internal", Internal(errors.New("boom")), codes.Internal, ReasonInternal, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.GRPCCode != tt.wantCode {
				t.Errorf("GRPCCode = %v, want %v", tt.err.GRPCCode, tt.wantCode)
			}
			if tt.err.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", tt.err.Reason, tt.wantReason)
			}
			if tt.err.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.wantMsg)
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
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", st.Code())
	}
	if !strings.Contains(appErr.Error(), "10.0.0.5") {
		t.Errorf("Error() should keep the cause for server logs, got %q", appErr.Error())
	}
	if !errors.Is(appErr, cause) {
		t.Error("errors.Is should reach the wrapped cause")
	}
}

func TestGRPCStatusCarriesErrorInfo(t *testing.T) {
	st := status.Convert(InvalidArgument("name is required"))

	if st.Code() != codes.InvalidArgument {
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
	if info.GetReason() != string(ReasonInvalidArgument) || info.GetDomain() != "orcan-api" {
		t.Errorf("ErrorInfo = {reason: %q, domain: %q}", info.GetReason(), info.GetDomain())
	}
}

func TestAs(t *testing.T) {
	notFound := NotFound("product", gorm.ErrRecordNotFound)

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

func TestErrorWithoutCause(t *testing.T) {
	if got := InvalidArgument("bad input").Error(); got != "bad input" {
		t.Errorf("Error() = %q", got)
	}
}
