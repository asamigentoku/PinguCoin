package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/config"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/interceptor"
	pb "github.com/asamigentoku/PinguCoin/server/payment-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/repository"
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

	logger.Info("payment-api (gRPC) listening", slog.String("port", cfg.Port))
	if err := server.Serve(listener); err != nil {
		logger.Error("failed to serve", slog.Any("error", err))
		os.Exit(1)
	}
}
