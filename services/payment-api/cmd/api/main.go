package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbretry"
	"github.com/asamigentoku/PinguCoin/pkg/health"
	"github.com/asamigentoku/PinguCoin/pkg/interceptor"
	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/version"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

func main() {
	// Datadog の標準属性にそろえた JSON のログ(service / env / version が、すべてのログに付く)。
	// 環境は APP_ENV、ログのレベルは LOG_LEVEL(debug / info / warn / error)で決める。
	logger := logging.NewFromEnv("payment-api")
	slog.SetDefault(logger)
	build := version.Get()
	logger.Info("payment-api starting",
		slog.Group("build", slog.String("commit", build.Commit), slog.String("time", build.BuildTime), slog.String("go", build.GoVersion)),
	)

	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}

	// DB が起動中・再起動中などで、一時的につながらないときは、バックオフつきで再試行する。
	// 設定の間違い(接続文字列・パスワード・DB名)は、再試行せず、すぐに終了する。
	db, err := dbretry.Connect(context.Background(), logger, func() (*gorm.DB, error) { return database.Connect(cfg, logger) })
	if err != nil {
		logger.Error("failed to connect database", logging.Err(err))
		os.Exit(1)
	}

	if err := database.Migrate(db); err != nil {
		logger.Error("failed to migrate database", logging.Err(err))
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("failed to listen", logging.Err(err))
		os.Exit(1)
	}

	// interceptor.Auth はpingu-api以外からの直接のgRPC呼び出しを拒否する(サービス間認証)。
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.RequestID(), // 呼び出し元(pingu-api)のリクエスト ID を受け取る(Logging より前)
			interceptor.Logging(logger),
			interceptor.Auth(cfg.InternalAPIToken),
		),
	)

	pointRepo := repository.NewPointRepository(db)

	pb.RegisterPaymentServiceServer(server, grpcserver.NewPaymentServer(
		db,
		repository.NewPaymentRepository(db),
		repository.NewRefundRepository(db),
		pointRepo,
	))
	pb.RegisterPointServiceServer(server, grpcserver.NewPointServer(pointRepo))

	reflection.Register(server)

	// ヘルスチェック(Kubernetesのprobe用)。SIGTERMを受けたら readiness を落として止める。
	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	health.Register(shutdownCtx, server, db, logger)
	go func() {
		<-shutdownCtx.Done()
		logger.Info("shutting down: draining in-flight requests")
		// 処理中のRPCが終わるのを待ってから止める(新しいRPCは受け付けない)。
		server.GracefulStop()
	}()

	logger.Info("payment-api (gRPC) listening", slog.String("port", cfg.Port))
	if err := server.Serve(listener); err != nil {
		logger.Error("failed to serve", logging.Err(err))
		os.Exit(1)
	}
}
