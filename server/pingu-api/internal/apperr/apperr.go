// Package apperr は pingu-api (GraphQL/REST gateway) 全体で使うアプリケーション固有のエラー定義。
//
// orcan-api/payment-apiとの通信はgRPCだが、pingu-apiが外部に見せるのはGraphQL/RESTなので、
// gRPCの codes.Code を HTTPステータス相当のReason/HTTPStatusへ変換してから返す。
// 元になったエラー(gRPCの生エラー等)はクライアントには一切返さず、サーバー側のログにのみ残す。
package apperr

import (
	"errors"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Reason はクライアントが分岐に使える機械可読なエラー理由。
type Reason string

const (
	ReasonNotFound           Reason = "NOT_FOUND"
	ReasonInvalidArgument    Reason = "INVALID_ARGUMENT"
	ReasonAlreadyExists      Reason = "ALREADY_EXISTS"
	ReasonUnauthenticated    Reason = "UNAUTHENTICATED"
	ReasonFailedPrecondition Reason = "FAILED_PRECONDITION"
	ReasonInternal           Reason = "INTERNAL"
)

// AppError はHTTPステータス + アプリ独自のReason + ログ専用の内部エラーをまとめて持つ。
type AppError struct {
	HTTPStatus int
	Reason     Reason
	Message    string // クライアントに返して安全なメッセージ
	Err        error  // ログ用の元エラー。クライアントのレスポンスには含めない。
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NotFound(resource string) *AppError {
	return &AppError{HTTPStatus: http.StatusNotFound, Reason: ReasonNotFound, Message: resource + " not found"}
}

func InvalidArgument(message string) *AppError {
	return &AppError{HTTPStatus: http.StatusBadRequest, Reason: ReasonInvalidArgument, Message: message}
}

func AlreadyExists(resource string) *AppError {
	return &AppError{HTTPStatus: http.StatusConflict, Reason: ReasonAlreadyExists, Message: resource + " already exists"}
}

func Unauthenticated(message string) *AppError {
	return &AppError{HTTPStatus: http.StatusUnauthorized, Reason: ReasonUnauthenticated, Message: message}
}

func FailedPrecondition(message string) *AppError {
	return &AppError{HTTPStatus: http.StatusConflict, Reason: ReasonFailedPrecondition, Message: message}
}

func Internal(err error) *AppError {
	return &AppError{HTTPStatus: http.StatusInternalServerError, Reason: ReasonInternal, Message: "internal server error", Err: err}
}

// As は err が *AppError かどうかを判定するヘルパー(errors.Asの薄いラッパー)。
func As(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}

// FromGRPC はorcan-api/payment-api(gRPC)から返ってきたエラーを、
// pingu-api側のAppErrorに変換する。google.rpc.ErrorInfoのReasonを引き継ぎ、
// gRPCのcodes.Codeに応じたHTTPステータスへマッピングする。
func FromGRPC(err error) *AppError {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return Internal(err)
	}

	reason := Reason(st.Code().String())
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			reason = Reason(info.GetReason())
		}
	}

	switch st.Code() {
	case codes.NotFound:
		return &AppError{HTTPStatus: http.StatusNotFound, Reason: reason, Message: st.Message()}
	case codes.InvalidArgument:
		return &AppError{HTTPStatus: http.StatusBadRequest, Reason: reason, Message: st.Message()}
	case codes.AlreadyExists:
		return &AppError{HTTPStatus: http.StatusConflict, Reason: reason, Message: st.Message()}
	case codes.Unauthenticated:
		return &AppError{HTTPStatus: http.StatusUnauthorized, Reason: reason, Message: st.Message()}
	case codes.FailedPrecondition:
		return &AppError{HTTPStatus: http.StatusConflict, Reason: reason, Message: st.Message()}
	default:
		return Internal(err)
	}
}
