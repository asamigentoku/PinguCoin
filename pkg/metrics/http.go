package metrics

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// unmatchedRoute は、どのルートにも合わなかったリクエストのラベル。生のパスは、ラベルに入れない
// (存在しない URL を、大量に叩かれて、時系列が際限なく増えるのを防ぐ)。
const unmatchedRoute = "unmatched"

var (
	httpRequests = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_http_requests_total",
		Help: "HTTP リクエストの数(メソッド・ルート・ステータスコード別)。",
	}, []string{"method", "route", "code"})

	httpDuration = factory().NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_http_request_duration_seconds",
		Help:    "HTTP リクエストの処理時間(秒)。認証など、API の手前の処理も含む。",
		Buckets: LatencyBuckets,
	}, []string{"method", "route"})

	httpResponseBytes = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_http_response_bytes_total",
		Help: "HTTP レスポンスの本文のバイト数の合計(ルート別)。",
	}, []string{"route"})

	httpInFlight = factory().NewGauge(prometheus.GaugeOpts{
		Name: "pingucoin_http_in_flight_requests",
		Help: "いま処理中の HTTP リクエストの数。",
	})
)

// routeHolder は、リクエストが、どのルート(パターン)に合ったかを、外側のミドルウェアへ伝える入れ物。
// ルートが決まるのは、いちばん内側の ServeMux なので、context に入れ物を置いて、あとで、中身を読む。
type routeHolder struct{ route string }

type routeKey struct{}

// HTTP は、HTTP のリクエストの数・ステータスコード・処理時間・処理中の数・レスポンスの大きさを測るミドルウェア。
// 認証・ログなどを含めて測れるよう、**できるだけ外側**に置く。ルートの名前は、内側に置いた Route から受け取る。
//
// ヘルスチェック(/healthz、/readyz)も、数に入る(ログと違って、数は軽いので、kubelet からの呼び出しの状態も見える)。
func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		holder := &routeHolder{}
		r = r.WithContext(context.WithValue(r.Context(), routeKey{}, holder))
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		httpInFlight.Inc()
		start := time.Now()
		defer func() {
			httpInFlight.Dec()
			route := holder.route
			if route == "" {
				route = unmatchedRoute
			}
			httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
			httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(recorder.status)).Inc()
			httpResponseBytes.WithLabelValues(route).Add(float64(recorder.bytes))
		}()

		next.ServeHTTP(recorder, r)
	})
}

// Route は、ServeMux(ルーター)を、直接包む。リクエストが合ったルートのパターン(例: "/api/v1/orders/{id}")を、
// HTTP ミドルウェアに伝える。パターンは、パスそのものではなく、`{id}` のままなので、ラベルの数が増えない。
//
// ServeMux は、渡されたリクエスト(r)の Pattern を、その場で書き込むので、r を作り直さずに、そのまま渡すこと。
func Route(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if holder, ok := r.Context().Value(routeKey{}).(*routeHolder); ok {
			holder.route = routeFromPattern(r.Pattern)
		}
	})
}

// routeFromPattern は、"POST /api/v1/orders" のようなパターンから、メソッドを除いて、パスの部分だけを返す。
// 合うパターンが無かった(空)ときは、"unmatched"。
func routeFromPattern(pattern string) string {
	if pattern == "" {
		return unmatchedRoute
	}
	if _, path, ok := strings.Cut(pattern, " "); ok {
		return path
	}
	return pattern
}

// responseRecorder は、書き込まれたステータスコードと、本文のバイト数を記録する。
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	r.wroteHeader = true
	written, err := r.ResponseWriter.Write(body)
	r.bytes += written
	return written, err
}

// Unwrap は、http.ResponseController が、元の ResponseWriter の機能(Flush など)を使えるようにする。
func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Flush は、ストリーミング(Server-Sent Events など)のために、元の Flusher に渡す。
func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
