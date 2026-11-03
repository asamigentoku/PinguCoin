package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orderclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/repository"
)

const APIVersionPrefix = "/api/v1"

func NewRouter(logger *slog.Logger, media *orcanclient.Client, payment *paymentclient.Client, orderRepo *repository.OrderRepository, ping func(context.Context) error) http.Handler {
	return newRouter(logger, media, payment, nil, orderRepo, ping)
}

func NewRouterWithOrderAPI(logger *slog.Logger, media *orcanclient.Client, payment *paymentclient.Client, orders *orderclient.Client, orderRepo *repository.OrderRepository, ping func(context.Context) error) http.Handler {
	return newRouter(logger, media, payment, orders, orderRepo, ping)
}

func newRouter(logger *slog.Logger, media *orcanclient.Client, payment *paymentclient.Client, orders *orderclient.Client, orderRepo *repository.OrderRepository, ping func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	health := NewHealthHandler(ping)
	mux.HandleFunc("GET /version", health.Version)
	mux.HandleFunc("GET /healthz", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)

	catalog := NewCatalogHandler(media, orders, orderRepo)
	mux.HandleFunc("GET "+APIVersionPrefix+"/me", catalog.Me)
	mux.HandleFunc("GET "+APIVersionPrefix+"/me/products", catalog.ListMyProducts)
	mux.HandleFunc("GET "+APIVersionPrefix+"/products", catalog.ListProducts)
	mux.HandleFunc("POST "+APIVersionPrefix+"/products", catalog.CreateProduct)
	mux.HandleFunc("GET "+APIVersionPrefix+"/products/{id}", catalog.GetProduct)
	mux.HandleFunc("PATCH "+APIVersionPrefix+"/products/{id}", catalog.UpdateProduct)
	mux.HandleFunc("DELETE "+APIVersionPrefix+"/products/{id}", catalog.DeleteProduct)
	mux.HandleFunc("GET "+APIVersionPrefix+"/products/{id}/assets", catalog.ListAssets)
	mux.HandleFunc("GET "+APIVersionPrefix+"/products/{id}/upload-targets", catalog.GetUploadTargets)
	mux.HandleFunc("POST "+APIVersionPrefix+"/products/{id}/image", catalog.ConfirmImage)
	mux.HandleFunc("DELETE "+APIVersionPrefix+"/products/{id}/image", catalog.DeleteImage)
	mux.HandleFunc("POST "+APIVersionPrefix+"/products/{id}/assets", catalog.ConfirmAsset)
	mux.HandleFunc("DELETE "+APIVersionPrefix+"/assets/{id}", catalog.DeleteAsset)
	mux.HandleFunc("GET "+APIVersionPrefix+"/assets/{id}/download", catalog.GetAssetDownload)

	if orders != nil {
		handler := NewOrderGatewayHandler(orders, media)
		mux.HandleFunc("POST "+APIVersionPrefix+"/orders", handler.CreateOrder)
		mux.HandleFunc("GET "+APIVersionPrefix+"/orders", handler.ListOrders)
		mux.HandleFunc("GET "+APIVersionPrefix+"/orders/{id}", handler.GetOrder)
	} else {
		handler := NewOrderHandler(logger, media, payment, orderRepo)
		mux.HandleFunc("POST "+APIVersionPrefix+"/orders", handler.CreateOrder)
		mux.HandleFunc("GET "+APIVersionPrefix+"/orders", handler.ListOrders)
		mux.HandleFunc("GET "+APIVersionPrefix+"/orders/{id}", handler.GetOrder)
	}
	points := NewPointHandler(payment)
	mux.HandleFunc("GET "+APIVersionPrefix+"/points", points.GetPoints)
	mux.HandleFunc("POST "+APIVersionPrefix+"/points/topups", points.TopUpPoints)
	mux.HandleFunc("GET "+APIVersionPrefix+"/payments/{id}", points.GetPayment)
	return metrics.HTTP(WithRequestID(WithAPIVersion(WithLogging(logger)(WithOptionalAuth(media)(metrics.Route(mux))))))
}
