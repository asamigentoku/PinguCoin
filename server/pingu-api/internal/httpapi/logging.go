package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder は http.ResponseWriter をラップし、実際に書き込まれたステータスコードを記録する
// (net/httpの標準APIにはレスポンス後からステータスを取得する手段がないため)。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

// WithLogging は全HTTPリクエスト(GraphQL/RESTの両方)を構造化ログに出すミドルウェア。
// orcan-api/payment-apiのinternal/interceptor.Logging(gRPC版)と同じ考え方で、
// 各ハンドラー側で個別にログを書かなくても、ここ一箇所でリクエスト単位のログを一元的に出す。
// Internal(5xx)エラーの詳細な原因は、発生箇所(internal/httpapi/response.go)側で別途ログする。
func WithLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Duration("duration", time.Since(start)),
			}

			switch {
			case recorder.status >= http.StatusInternalServerError:
				logger.LogAttrs(r.Context(), slog.LevelError, "http request failed", attrs...)
			case recorder.status >= http.StatusBadRequest:
				logger.LogAttrs(r.Context(), slog.LevelWarn, "http request rejected", attrs...)
			default:
				logger.LogAttrs(r.Context(), slog.LevelInfo, "http request completed", attrs...)
			}
		})
	}
}
