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

	"github.com/asamigentoku/PinguCoin/pkg/health"
	"github.com/asamigentoku/PinguCoin/pkg/interceptor"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}

	db, err := database.Connect(cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}

	if err := database.AutoMigrate(db); err != nil {
		logger.Error("failed to migrate database", slog.Any("error", err))
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("failed to listen", slog.Any("error", err))
		os.Exit(1)
	}

	// interceptor.Auth はpingu-api以外からの直接のgRPC呼び出しを拒否する(サービス間認証)。
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
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
		logger.Error("failed to serve", slog.Any("error", err))
		os.Exit(1)
	}
}
