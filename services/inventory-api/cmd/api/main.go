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
	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/eventconsumer"
	server "github.com/asamigentoku/PinguCoin/services/inventory-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

func main() {
	logger := logging.NewFromEnv("inventory-api")
	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Error("database connection failed", logging.Err(err))
		os.Exit(1)
	}
	if err = database.Migrate(db); err != nil {
		logger.Error("migration failed", logging.Err(err))
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("listen failed", logging.Err(err))
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(metrics.UnaryServerInterceptor(), interceptor.RequestID(), interceptor.Logging(logger), interceptor.Auth(cfg.InternalAPIToken)))
	pb.RegisterProductInventoryServiceServer(grpcServer, server.New(repository.New(db)))
	consumer, err := eventconsumer.New(cfg.KafkaBrokers, repository.New(db), logger)
	if err != nil {
		logger.Error("kafka consumer failed", logging.Err(err))
		os.Exit(1)
	}
	defer consumer.Close()
	reflection.Register(grpcServer)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics.Init("inventory-api")
	go metrics.Serve(ctx, metrics.Addr(), logger)
	go func() {
		if err := consumer.Run(ctx); err != nil {
			logger.Error("inventory consumer stopped after retries", logging.Err(err))
			stop()
		}
	}()
	health.Register(ctx, grpcServer, db, logger)
	go func() { <-ctx.Done(); grpcServer.GracefulStop() }()
	logger.Info("inventory-api listening", slog.String("port", cfg.Port))
	if err = grpcServer.Serve(listener); err != nil {
		logger.Error("serve failed", logging.Err(err))
	}
}
