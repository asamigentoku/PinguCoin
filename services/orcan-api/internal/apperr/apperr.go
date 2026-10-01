// Package apperr は orcan-api 全体で使うアプリケーション固有のエラー定義。
//
// gRPCの codes.Code だけだと「NotFound」が具体的に何のリソースの話か
// クライアント側で機械的に判別しづらいため、文字列の Reason(例: "PRODUCT_NOT_FOUND")を
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
	ReasonPermissionDenied   Reason = "PERMISSION_DENIED"
	ReasonFailedPrecondition Reason = "FAILED_PRECONDITION"
	ReasonInternal           Reason = "INTERNAL"
)

const domain = "orcan-api"

// AppError はgRPCの codes.Code + アプリ独自のReason + ログ専用の内部エラーをまとめて持つ。
// クライアントに返して問題ない情報(GRPCCode, Reason, Message)と、
// サーバーログにのみ出す情報(Err)を明確に分ける。
type AppError struct {
	GRPCCode codes.Code
	Reason   Reason
	Message  string // クライアントに返して安全なメッセージ
	Err      error  // ログ用の元エラー。クライアントのレスポンスには含めない。
}

func (appErr *AppError) Error() string {
	if appErr.Err != nil {
		return appErr.Message + ": " + appErr.Err.Error()
	}
	return appErr.Message
}

func (appErr *AppError) Unwrap() error {
	return appErr.Err
}

// GRPCStatus を実装すると、grpc-go の status.FromError / status.Convert が
// このErrorInfo付きのステータスをそのまま使ってくれる。
func (appErr *AppError) GRPCStatus() *status.Status {
	st := status.New(appErr.GRPCCode, appErr.Message)
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: string(appErr.Reason),
		Domain: domain,
	})
	if err != nil {
		return st
	}
	return withDetails
}

// NotFound は指定したリソースが見つからなかったことを表す。
// resource には "product", "product category" のような表示名を渡す。
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

// PermissionDenied はリソースへのアクセス権限が無いことを表す(例: 他人の商品を操作しようとした)。
func PermissionDenied(message string) *AppError {
	return &AppError{
		GRPCCode: codes.PermissionDenied,
		Reason:   ReasonPermissionDenied,
		Message:  message,
	}
}

// FailedPrecondition はリソースの現在の状態的に処理を続行できないことを表す
// (例: 在庫不足)。
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
