// Package apperr は payment-api 全体で使うアプリケーション固有のエラー定義。
//
// gRPCの codes.Code だけだと「NotFound」が具体的に何のリソースの話か
// クライアント側で機械的に判別しづらいため、文字列の Reason(例: "PAYMENT_NOT_FOUND")を
// google.rpc.ErrorInfo として付与する。また元になったエラー(DBのエラーなど)は
// クライアントには一切返さず、サーバー側のログにのみ残す(内部実装の詳細を漏らさないため)。
package apperr

import (
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Reason はクライアントが分岐に使える機械可読なエラー理由。
type Reason string

const (
	ReasonNotFound           Reason = "NOT_FOUND"
	ReasonInvalidArgument    Reason = "INVALID_ARGUMENT"
	ReasonFailedPrecondition Reason = "FAILED_PRECONDITION"
	ReasonInternal           Reason = "INTERNAL"
)

const domain = "payment-api"

// AppError はgRPCの codes.Code + アプリ独自のReason + ログ専用の内部エラーをまとめて持つ。
// クライアントに返して問題ない情報(GRPCCode, Reason, Message)と、
// サーバーログにのみ出す情報(Err)を明確に分ける。
type AppError struct {
	GRPCCode codes.Code
	Reason   Reason
	Message  string // クライアントに返して安全なメッセージ
	Err      error  // ログ用の元エラー。クライアントのレスポンスには含めない。
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

// GRPCStatus を実装すると、grpc-go の status.FromError / status.Convert が
// このErrorInfo付きのステータスをそのまま使ってくれる。
func (e *AppError) GRPCStatus() *status.Status {
	st := status.New(e.GRPCCode, e.Message)
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: string(e.Reason),
		Domain: domain,
	})
	if err != nil {
		return st
	}
	return withDetails
}

// NotFound は指定したリソースが見つからなかったことを表す。
// resource には "payment", "refund" のような表示名を渡す。
func NotFound(resource string, err error) *AppError {
	return &AppError{
		GRPCCode: codes.NotFound,
		Reason:   ReasonNotFound,
		Message:  resource + " not found",
		Err:      err,
	}
}

// InvalidArgument はリクエストの入力値が不正であることを表す(バリデーションエラー)。
func InvalidArgument(message string) *AppError {
	return &AppError{
		GRPCCode: codes.InvalidArgument,
		Reason:   ReasonInvalidArgument,
		Message:  message,
	}
}

// FailedPrecondition は状態遷移として許されない操作(例: 完了済み決済のキャンセル)を表す。
func FailedPrecondition(message string) *AppError {
	return &AppError{
		GRPCCode: codes.FailedPrecondition,
		Reason:   ReasonFailedPrecondition,
		Message:  message,
	}
}

// Internal はDBエラーなど、クライアントに詳細を返すべきではない内部エラーを表す。
// 元エラー(err)はログに出すためだけに保持し、クライアントには汎用メッセージのみ返す。
func Internal(err error) *AppError {
	return &AppError{
		GRPCCode: codes.Internal,
		Reason:   ReasonInternal,
		Message:  "internal server error",
		Err:      err,
	}
}

// As は err が *AppError かどうかを判定するヘルパー(errors.Asの薄いラッパー)。
func As(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}
