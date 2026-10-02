// Package metrics は、3つのサービス(orcan-api / payment-api / pingu-api)共通の、Prometheus 向けのメトリクスを提供する。
//
// メトリクスは、ログ(1件ずつの出来事)と違って、「数」の推移(1秒あたりのリクエスト数、エラーの割合、処理時間の分布、
// DB の接続の使用状況など)を、安く、長く持つためのもの。Prometheus が、各 Pod の /metrics を定期的に取りに来て(scrape)、
// Grafana が、それをグラフにする(docs/MONITORING.md)。
//
// 出すメトリクス(すべて pingucoin_ で始まる。Go のランタイムとプロセスのメトリクスは、go_ / process_ のまま)
//
//	pingucoin_build_info                      ... バージョン(値は常に 1。ラベルに service / version / commit)
//	pingucoin_http_*                          ... HTTP サーバー(pingu-api)               http.go
//	pingucoin_graphql_*                       ... GraphQL の操作(pingu-api)              services/pingu-api/internal/httpapi
//	pingucoin_grpc_server_*                   ... gRPC サーバー(orcan-api / payment-api)  grpc.go
//	pingucoin_grpc_client_*                   ... gRPC クライアント(pingu-api → 他の2つ)   grpc.go
//	pingucoin_db_*                            ... DB の接続プールと、クエリ                 db.go
//	pingucoin_orders_* / payments_* / ...     ... 業務の数(注文・決済・ポイント・在庫・Blob) business.go
//
// 守ること(カーディナリティ = ラベルの値の組み合わせの数)
//   - ラベルの値は、有限で、小さい集合にする。ユーザー ID、商品 ID、リクエスト ID、生の URL のパス、
//     クライアントが決められる文字列(GraphQL の操作名など)は、ラベルに入れない。入れると、時系列が際限なく増えて、
//     Prometheus のメモリとディスクを食いつぶす(利用者が、意図せず、または悪意で、増やせてしまう)。
//   - 値の種類が決まっているもの(HTTP のメソッド、ルートのパターン、gRPC のメソッド、ステータスコード、結果)だけをラベルにする。
//
// メトリクスは、**公開しない**。別のポート(既定 9090。環境変数 METRICS_PORT)で待ち受け、クラスターの中の Prometheus だけが取る
// (NetworkPolicy で、それ以外からは届かない。platform/kubernetes/*/monitoring)。
package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/version"
)

// DefaultPort は、メトリクスを出すポート。API 本体のポート(8080 / 8081 / 8082)とは別にする。
const DefaultPort = "9090"

// LatencyBuckets は、API(HTTP / gRPC)の処理時間のヒストグラムの区切り(秒)。10 ms から 10 秒まで。
var LatencyBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// registerer は、メトリクスを登録する先(プロセスで1つ)。Go のランタイム(go_*)とプロセス(process_*)のメトリクスは、
// prometheus が、最初から登録している。
var registerer prometheus.Registerer = prometheus.DefaultRegisterer

// factory は、registerer に登録する promauto のファクトリー。
func factory() promauto.Factory { return promauto.With(registerer) }

// Init は、プロセスの起動時に、1回呼ぶ。バージョン情報のメトリクスを登録する。
// 「いまどのバージョンが動いているか」「デプロイで切り替わった時刻」を、グラフに重ねて見られる。
func Init(service string) {
	info := version.Get()
	buildInfo := factory().NewGaugeVec(prometheus.GaugeOpts{
		Name: "pingucoin_build_info",
		Help: "ビルドの情報。値は常に 1。",
	}, []string{"service", "version", "commit", "go_version"})
	buildInfo.WithLabelValues(service, info.Version, info.Commit, info.GoVersion).Set(1)
}

// Handler は、/metrics の中身を返す HTTP ハンドラー。
func Handler() http.Handler {
	return promhttp.Handler()
}

// Addr は、メトリクスを待ち受けるアドレス(":9090")。環境変数 METRICS_PORT で変えられる。
func Addr() string {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = DefaultPort
	}
	return ":" + port
}

// Serve は、メトリクス専用の HTTP サーバー(GET /metrics だけ)を起動する。ctx が終わったら止める。ブロックするので、goroutine で呼ぶ。
//
// API 本体とは別のポート・別のサーバーにして、メトリクスが、インターネットに向けた入口(Ingress)から見えないようにする。
// メトリクス用のサーバーが失敗しても、API 本体は止めない(ログに出すだけ。監視の不具合で、本番のサービスを止めない)。
func Serve(ctx context.Context, addr string, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler())

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("metrics listening", slog.String("addr", addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server stopped", logging.Err(err))
	}
}
