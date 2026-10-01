package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/requestid"
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

// WithRequestID は、リクエスト ID(ヘッダー X-Request-Id)を、context とレスポンスのヘッダーに持たせる。
// リクエストに ID があればそれを使い(使えない文字を含む場合は作り直し)、なければ作る。いちばん外側に置くこと。
// pingu-api が gRPC で呼ぶときも、同じ ID を渡すので(grpcclient)、orcan-api / payment-api のログとも、同じ request_id でつながる。
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.OrNew(r.Header.Get(requestid.Header))
		w.Header().Set(requestid.Header, id)
		next.ServeHTTP(w, r.WithContext(requestid.WithContext(r.Context(), id)))
	})
}

// WithLogging は全HTTPリクエスト(GraphQL/RESTの両方)を構造化ログに出すミドルウェア。
// orcan-api/payment-apiのpkg/interceptor.Logging(gRPC版)と同じ考え方で、
// 各ハンドラー側で個別にログを書かなくても、ここ一箇所でリクエスト単位のログを一元的に出す。
// Internal(5xx)エラーの詳細な原因は、発生箇所(internal/httpapi/response.go)側で別途ログする。
//
// ログの属性は、Datadog の標準属性にそろえる(pkg/logging を参照)。
//
//	http.method / http.url / http.status_code / http.useragent / http.referer / http.version
//	network.client.ip,  duration(ナノ秒),  request_id
//
// http.url には、パスだけを出す(クエリ文字列は、トークンなどの秘密を含みうるので、出さない)。
// リクエストの本文(GraphQL のクエリや変数)も、出さない。
func WithLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isHealthCheck(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			attrs := []slog.Attr{
				slog.Group("http",
					slog.String("method", r.Method),
					slog.String("url", r.URL.Path),
					slog.Int("status_code", recorder.status),
					slog.String("useragent", r.UserAgent()),
					slog.String("referer", r.Referer()),
					slog.String("version", r.Proto),
				),
				slog.Group("network", slog.Group("client", slog.String("ip", clientIP(r)))),
				logging.Duration(time.Since(start)),
				logging.RequestID(r.Context()),
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

// clientIP は、クライアントの IP。プロキシ(Ingress)の後ろでは、X-Forwarded-For の先頭(元のクライアント)を使う。
// ヘッダーは偽装できるので、ログ用の参考値であり、認可には使わないこと。
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if ip := strings.TrimSpace(first); net.ParseIP(ip) != nil {
			return ip
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
