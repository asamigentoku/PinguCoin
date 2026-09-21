package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/model"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/paymentclient"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
	paymentpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/reqcontext"
)

// OrderHandler は「商品購入(注文API)」を扱う。
// 決済(Payment)はpayment-apiに委譲し、その結果を注文(Order)としてpingu-api自身のDBに記録する。
// Order と Payment は別エンティティとして保持し、PaymentIDで紐づける。
type OrderHandler struct {
	orcan   *orcanclient.Client
	payment *paymentclient.Client
	repo    *repository.OrderRepository
}

func NewOrderHandler(orcan *orcanclient.Client, payment *paymentclient.Client, repo *repository.OrderRepository) *OrderHandler {
	return &OrderHandler{orcan: orcan, payment: payment, repo: repo}
}

type createOrderRequest struct {
	ProductID     uint32 `json:"product_id"`
	Quantity      int64  `json:"quantity"`
	PaymentMethod string `json:"payment_method"`
}

type orderResponse struct {
	ID          uint   `json:"id"`
	UserID      uint   `json:"user_id"`
	ProductID   uint   `json:"product_id"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	TotalAmount int64  `json:"total_amount"`
	PaymentID   uint   `json:"payment_id"`
	Status      string `json:"status"`
}

func toOrderResponse(o *model.Order) orderResponse {
	return orderResponse{
		ID:          o.ID,
		UserID:      o.UserID,
		ProductID:   o.ProductID,
		Quantity:    o.Quantity,
		UnitPrice:   o.UnitPrice,
		TotalAmount: o.TotalAmount,
		PaymentID:   o.PaymentID,
		Status:      o.Status,
	}
}

// CreateOrder は POST /orders。ログイン中ユーザーが買い手(user_id)となる。
// 1) orcan-apiで商品を確認 → 2) payment-apiで決済 → 3) 注文としてDBに記録、の順で処理する。
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, apperr.Unauthenticated("login is required"))
		return
	}

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperr.InvalidArgument("invalid request body"))
		return
	}
	if req.ProductID == 0 {
		writeError(w, apperr.InvalidArgument("product_id is required"))
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.PaymentMethod == "" {
		req.PaymentMethod = "point"
	}

	productResp, err := h.orcan.Product.GetProduct(r.Context(), &orcanpb.GetProductRequest{Id: req.ProductID})
	if err != nil {
		writeError(w, apperr.FromGRPC(err))
		return
	}
	product := productResp.GetProduct()

	totalAmount := product.GetPrice() * req.Quantity

	paymentResp, err := h.payment.Payment.CreatePayment(r.Context(), &paymentpb.CreatePaymentRequest{
		UserId:        uint32(claims.UserID),
		ProductId:     req.ProductID,
		Amount:        totalAmount,
		Currency:      "JPY",
		PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		writeError(w, apperr.FromGRPC(err))
		return
	}
	payment := paymentResp.GetPayment()

	order := &model.Order{
		UserID:      claims.UserID,
		ProductID:   uint(req.ProductID),
		Quantity:    req.Quantity,
		UnitPrice:   product.GetPrice(),
		TotalAmount: totalAmount,
		PaymentID:   uint(payment.GetId()),
		Status:      orderStatusFromPayment(payment.GetStatus()),
	}
	if err := h.repo.Create(order); err != nil {
		writeError(w, apperr.Internal(err))
		return
	}

	writeJSON(w, http.StatusCreated, toOrderResponse(order))
}

// ListOrders は GET /orders。ログイン中ユーザー自身の注文一覧を返す。
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, apperr.Unauthenticated("login is required"))
		return
	}

	orders, err := h.repo.FindByUser(claims.UserID)
	if err != nil {
		writeError(w, apperr.Internal(err))
		return
	}

	resp := make([]orderResponse, 0, len(orders))
	for i := range orders {
		resp = append(resp, toOrderResponse(&orders[i]))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetOrder は GET /orders/{id}。他ユーザーの注文は(存在を推測されないよう)404として扱う。
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, apperr.Unauthenticated("login is required"))
		return
	}

	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, apperr.InvalidArgument("invalid order id"))
		return
	}

	order, err := h.repo.FindByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, apperr.NotFound("order"))
			return
		}
		writeError(w, apperr.Internal(err))
		return
	}
	if order.UserID != claims.UserID {
		writeError(w, apperr.NotFound("order"))
		return
	}

	writeJSON(w, http.StatusOK, toOrderResponse(order))
}

func orderStatusFromPayment(paymentStatus string) string {
	switch paymentStatus {
	case "succeeded":
		return "paid"
	case "pending":
		return "pending"
	default:
		return "failed"
	}
}
