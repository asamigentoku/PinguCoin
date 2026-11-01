package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/pkg/version"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/clerkauth"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/config"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/httpapi"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orderclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/paymentclient"
)

func main() {
	// Datadog の標準属性にそろえた JSON のログ(service / env / version が、すべてのログに付く)。
	// 環境は APP_ENV、ログのレベルは LOG_LEVEL(debug / info / warn / error)で決める。
	logger := logging.NewFromEnv("bff")
	slog.SetDefault(logger)
	build := version.Get()
	metrics.Init("bff")
	logger.Info("bff starting",
		slog.Group("build", slog.String("commit", build.Commit), slog.String("time", build.BuildTime), slog.String("go", build.GoVersion)),
	)

	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}
	clerkauth.Init(cfg.ClerkSecretKey)

	// DB が起動中・再起動中などで、一時的につながらないときは、バックオフつきで再試行する。
	// 設定の間違い(接続文字列・パスワード・DB名)は、再試行せず、すぐに終了する。
	orcan, err := orcanclient.New(cfg.OrcanAddr, cfg.InternalAPIToken)
	if err != nil {
		logger.Error("failed to connect media-api", logging.Err(err))
		os.Exit(1)
	}
	defer orcan.Close()
	orders, err := orderclient.New(cfg.OrderAddr, cfg.InternalAPIToken)
	if err != nil {
		logger.Error("failed to connect order-api", logging.Err(err))
		os.Exit(1)
	}
	defer orders.Close()

	payment, err := paymentclient.New(cfg.PaymentAddr, cfg.InternalAPIToken)
	if err != nil {
		logger.Error("failed to connect payment-api", logging.Err(err))
		os.Exit(1)
	}
	//main関数が終了後に実行することを定義
	defer payment.Close()

	router := httpapi.NewRouterWithOrderAPI(logger, orcan, payment, orders, nil, func(context.Context) error { return nil })

	logger.Info("bff (REST) listening",
		slog.String("port", cfg.Port),
		slog.String("orcan_addr", cfg.OrcanAddr),
		slog.String("payment_addr", cfg.PaymentAddr),
	)
	// タイムアウトを付けたサーバー。付けないと、遅い・応答しないクライアントが、接続と goroutine を、いつまでも占有する。
	//   - ReadHeaderTimeout ... ヘッダーを送り終えるまで(遅いクライアントを切る)
	//   - WriteTimeout     ... レスポンスを返し終えるまで。gRPC の期限(grpcclient.DefaultTimeout = 10 秒)より長くする
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// SIGTERM(Kubernetes が Pod を止めるときの合図)を受けたら、処理中のリクエストが終わるのを待ってから止める。
	// 待つのは最大 25 秒(terminationGracePeriodSeconds の 30 秒より短く)。新しいリクエストは、受け付けない。
	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	// メトリクス(/metrics)。API とは別のポートで待ち受け、クラスター内の Prometheus だけが取る(pkg/metrics)。
	go metrics.Serve(shutdownCtx, metrics.Addr(), logger)
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-shutdownCtx.Done()
		logger.Info("shutting down: draining in-flight requests")
		drainCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := server.Shutdown(drainCtx); err != nil {
			logger.Error("graceful shutdown did not finish", logging.Err(err))
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("failed to serve", logging.Err(err))
		os.Exit(1)
	}
	<-shutdownDone
}
