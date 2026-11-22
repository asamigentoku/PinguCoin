package main

import (
	"context"
	"github.com/asamigentoku/PinguCoin/pkg/health"
	"github.com/asamigentoku/PinguCoin/pkg/interceptor"
	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/eventconsumer"
	server "github.com/asamigentoku/PinguCoin/services/order-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/outbox"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// main は注文サービスを起動するエントリポイントです。
// 注文を受け付けるgRPCサーバーに加えて、注文イベントの発行と、
// 在庫・決済結果イベントの受信を行う非同期処理も起動します。
func main() {
	logger := logging.NewFromEnv("order-api")
	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}
	db, e := database.Connect(cfg.DatabaseURL)
	if e != nil {
		logger.Error("database failed", logging.Err(e))
		os.Exit(1)
	}
	if e = database.Migrate(db); e != nil {
		logger.Error("migration failed", logging.Err(e))
		os.Exit(1)
	}
	// 注文保存と同じDBを監視し、Kafkaへのイベント発行を担当します。
	publisher, e := outbox.New(db, cfg.KafkaBrokers, logger)
	if e != nil {
		logger.Error("kafka client failed", logging.Err(e))
		os.Exit(1)
	}
	defer publisher.Close()
	// 在庫・決済サービスから返ってくる結果イベントを購読します。
	consumer, e := eventconsumer.New(cfg.KafkaBrokers, repository.New(db), logger)
	if e != nil {
		logger.Error("kafka consumer failed", logging.Err(e))
		os.Exit(1)
	}
	defer consumer.Close()
	listener, e := net.Listen("tcp", ":"+cfg.Port)
	if e != nil {
		logger.Error("listen failed", logging.Err(e))
		os.Exit(1)
	}
	// 注文の作成・参照はgRPC APIとして提供し、認証・ログ・メトリクスを共通適用します。
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(metrics.UnaryServerInterceptor(), interceptor.RequestID(), interceptor.Logging(logger), interceptor.Auth(cfg.InternalAPIToken)))
	pb.RegisterOrderServiceServer(grpcServer, server.New(repository.New(db), cfg.KafkaTopic))
	reflection.Register(grpcServer)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics.Init("order-api")
	go metrics.Serve(ctx, metrics.Addr(), logger)
	go publisher.Run(ctx)
	go func() {
		if err := consumer.Run(ctx); err != nil {
			logger.Error("order consumer stopped after retries", logging.Err(err))
			stop()
		}
	}()
	health.Register(ctx, grpcServer, db, logger)
	go func() { <-ctx.Done(); grpcServer.GracefulStop() }()
	logger.Info("order-api listening", slog.String("port", cfg.Port))
	if e = grpcServer.Serve(listener); e != nil {
		logger.Error("serve failed", logging.Err(e))
	}
}
