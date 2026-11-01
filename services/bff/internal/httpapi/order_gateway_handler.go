package httpapi

import (
	"encoding/json"
	mediapb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
	pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orderclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/reqcontext"
	"net/http"
	"strconv"
	"strings"
)

type OrderGatewayHandler struct {
	client *orderclient.Client
	media  *orcanclient.Client
}

func NewOrderGatewayHandler(client *orderclient.Client, media *orcanclient.Client) *OrderGatewayHandler {
	return &OrderGatewayHandler{client: client, media: media}
}
func (h *OrderGatewayHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthenticated("login is required"))
		return
	}
	key := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader))
	if key == "" {
		writeError(w, r, apperr.InvalidArgument(idempotencyKeyHeader+" header is required"))
		return
	}
	var body createOrderRequest
	if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
		writeError(w, r, apperr.InvalidArgument("invalid request body"))
		return
	}
	productResponse, e := h.media.Product.GetProduct(r.Context(), &mediapb.GetProductRequest{Id: body.ProductID})
	if e != nil {
		writeError(w, r, apperr.FromGRPC(e))
		return
	}
	response, e := h.client.Order.CreateOrder(r.Context(), &pb.CreateOrderRequest{UserId: uint64(claims.UserID), ProductId: uint64(body.ProductID), Quantity: body.Quantity, PaymentMethod: body.PaymentMethod, IdempotencyKey: key, UnitPrice: productResponse.GetProduct().GetPrice()})
	if e != nil {
		writeError(w, r, apperr.FromGRPC(e))
		return
	}
	code := http.StatusCreated
	if response.GetReplayed() {
		code = http.StatusOK
	}
	writeJSON(w, code, gatewayOrder(response.GetOrder()))
}
func (h *OrderGatewayHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthenticated("login is required"))
		return
	}
	response, e := h.client.Order.ListOrders(r.Context(), &pb.ListOrdersRequest{UserId: uint64(claims.UserID)})
	if e != nil {
		writeError(w, r, apperr.FromGRPC(e))
		return
	}
	orders := make([]orderResponse, 0, len(response.GetOrders()))
	for _, order := range response.GetOrders() {
		orders = append(orders, gatewayOrder(order))
	}
	writeJSON(w, http.StatusOK, orders)
}
func (h *OrderGatewayHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthenticated("login is required"))
		return
	}
	id, e := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if e != nil {
		writeError(w, r, apperr.InvalidArgument("invalid order id"))
		return
	}
	response, e := h.client.Order.GetOrder(r.Context(), &pb.GetOrderRequest{Id: id, UserId: uint64(claims.UserID)})
	if e != nil {
		writeError(w, r, apperr.FromGRPC(e))
		return
	}
	writeJSON(w, http.StatusOK, gatewayOrder(response.GetOrder()))
}
func gatewayOrder(order *pb.Order) orderResponse {
	return orderResponse{ID: uint(order.GetId()), UserID: uint(order.GetUserId()), ProductID: uint(order.GetProductId()), Quantity: order.GetQuantity(), UnitPrice: order.GetUnitPrice(), TotalAmount: order.GetTotalAmount(), PaymentID: uint(order.GetPaymentId()), Status: order.GetStatus()}
}
