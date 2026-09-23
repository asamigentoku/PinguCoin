package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/clerkauth"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/config"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/httpapi"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}
	clerkauth.Init(cfg.ClerkSecretKey)

	db, err := database.Connect(cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}

	if err := database.AutoMigrate(db); err != nil {
		logger.Error("failed to migrate database", slog.Any("error", err))
		os.Exit(1)
	}

	orcan, err := orcanclient.New(cfg.OrcanAddr, cfg.InternalAPIToken)
	if err != nil {
		logger.Error("failed to connect orcan-api", slog.Any("error", err))
		os.Exit(1)
	}
	defer orcan.Close()

	payment, err := paymentclient.New(cfg.PaymentAddr, cfg.InternalAPIToken)
	if err != nil {
		logger.Error("failed to connect payment-api", slog.Any("error", err))
		os.Exit(1)
	}
	//main関数が終了後に実行することを定義
	defer payment.Close()

	orderRepo := repository.NewOrderRepository(db)

	router := httpapi.NewRouter(logger, orcan, payment, orderRepo)

	logger.Info("pingu-api (GraphQL/REST) listening",
		slog.String("port", cfg.Port),
		slog.String("orcan_addr", cfg.OrcanAddr),
		slog.String("payment_addr", cfg.PaymentAddr),
	)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		logger.Error("failed to serve", slog.Any("error", err))
		os.Exit(1)
	}
}
