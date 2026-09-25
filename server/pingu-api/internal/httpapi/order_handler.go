package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

// idempotencyKeyHeader はフロントエンドが二重注文防止のために発行する冪等性キーのヘッダ名。
// 同じキーでの再送(ネットワークエラー時のリトライ、二重クリック等)は新たに注文/決済/在庫消費を
// 作らず、最初に処理した結果をそのまま返す。orcan-api(在庫)・payment-api(決済)にも
// 同じキーをそのまま渡し、それぞれの側でも二重処理を防ぐ。
const idempotencyKeyHeader = "Idempotency-Key"

// inventoryReleaseSuffix は決済失敗時に在庫を戻す補償操作用のキーを、元のidempotency_keyから
// 派生させるための接尾辞。在庫消費とは別の操作として扱う必要があるため、そのまま使い回さない。
const inventoryReleaseSuffix = ":release"

// OrderHandler は「商品購入(注文API)」を扱う。
// 決済(Payment)はpayment-apiに、在庫の消費はorcan-apiに委譲し、その結果を注文(Order)として
// pingu-api自身のDBに記録する。Order と Payment は別エンティティとして保持し、PaymentIDで紐づける。
type OrderHandler struct {
	orcan   *orcanclient.Client
	payment *paymentclient.Client
	repo    *repository.OrderRepository
	logger  *slog.Logger
}

func NewOrderHandler(logger *slog.Logger, orcan *orcanclient.Client, payment *paymentclient.Client, repo *repository.OrderRepository) *OrderHandler {
	return &OrderHandler{orcan: orcan, payment: payment, repo: repo, logger: logger}
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

func toOrderResponse(order *model.Order) orderResponse {
	return orderResponse{
		ID:          order.ID,
		UserID:      order.UserID,
		ProductID:   order.ProductID,
		Quantity:    order.Quantity,
		UnitPrice:   order.UnitPrice,
		TotalAmount: order.TotalAmount,
		PaymentID:   order.PaymentID,
		Status:      order.Status,
	}
}

// CreateOrder は POST /api/v1/orders。ログイン中ユーザーが買い手(user_id)となる。
// 1) orcan-apiで商品を確認 → 2) orcan-apiで在庫を消費 → 3) payment-apiで決済 →
// 4) 注文としてDBに記録、の順で処理する。
//
// 冪等性: クライアントは Idempotency-Key ヘッダを必ず付ける。同じキーでの再送(リトライ・
// 二重クリック等)は、新たに注文/決済/在庫消費を作らず最初に処理した結果をそのまま返す。
// 在庫消費・決済作成にも同じキーをそのまま渡し、orcan-api/payment-api側でもそれぞれ
// 冪等性を担保している(このハンドラの2重送信対策だけに頼らない)。
func (handler *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, apperr.Unauthenticated("login is required"))
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader))
	if idempotencyKey == "" {
		writeError(w, apperr.InvalidArgument(idempotencyKeyHeader+" header is required"))
		return
	}

	// 既に同じキーで処理済みならそれをそのまま返す(新たな在庫消費/決済/注文作成は一切行わない)。
	if existing, err := handler.repo.FindByIdempotencyKey(idempotencyKey); err == nil {
		writeJSON(w, http.StatusOK, toOrderResponse(existing))
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, apperr.Internal(err))
		return
	}

	var request createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, apperr.InvalidArgument("invalid request body"))
		return
	}
	if request.ProductID == 0 {
		writeError(w, apperr.InvalidArgument("product_id is required"))
		return
	}
	if request.Quantity <= 0 {
		request.Quantity = 1
	}
	if request.PaymentMethod == "" {
		request.PaymentMethod = "point"
	}

	productResponse, err := handler.orcan.Product.GetProduct(r.Context(), &orcanpb.GetProductRequest{Id: request.ProductID})
	if err != nil {
		writeError(w, apperr.FromGRPC(err))
		return
	}
	product := productResponse.GetProduct()

	totalAmount := product.GetPrice() * request.Quantity

	// 在庫を消費する。在庫不足ならFailedPrecondition(409相当)が返り、ここで打ち切る
	// (決済前に確認するので、在庫不足の商品に課金してしまうことがない)。
	_, err = handler.orcan.Inventory.AdjustProductInventory(r.Context(), &orcanpb.AdjustProductInventoryRequest{
		ProductId:      request.ProductID,
		Amount:         -int32(request.Quantity),
		Reason:         "order",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		writeError(w, apperr.FromGRPC(err))
		return
	}

	paymentResponse, err := handler.payment.Payment.CreatePayment(r.Context(), &paymentpb.CreatePaymentRequest{
		UserId:         uint32(claims.UserID),
		ProductId:      request.ProductID,
		Amount:         totalAmount,
		Currency:       "JPY",
		PaymentMethod:  request.PaymentMethod,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		// 決済に失敗した場合、消費済みの在庫を戻す(ベストエフォート。これ自体が失敗しても
		// 決済エラーの方をクライアントに返す。在庫消費と別の操作なので専用のキーを使う)。
		_, releaseErr := handler.orcan.Inventory.AdjustProductInventory(r.Context(), &orcanpb.AdjustProductInventoryRequest{
			ProductId:      request.ProductID,
			Amount:         int32(request.Quantity),
			Reason:         "order payment failed",
			IdempotencyKey: idempotencyKey + inventoryReleaseSuffix,
		})
		if releaseErr != nil {
			handler.logger.Error("failed to release inventory after payment failure",
				slog.String("idempotency_key", idempotencyKey),
				slog.Any("payment_error", err),
				slog.Any("release_error", releaseErr),
			)
		}
		writeError(w, apperr.FromGRPC(err))
		return
	}
	payment := paymentResponse.GetPayment()

	order := &model.Order{
		UserID:         claims.UserID,
		ProductID:      uint(request.ProductID),
		Quantity:       request.Quantity,
		UnitPrice:      product.GetPrice(),
		TotalAmount:    totalAmount,
		PaymentID:      uint(payment.GetId()),
		Status:         orderStatusFromPayment(payment.GetStatus()),
		IdempotencyKey: idempotencyKey,
	}
	if err := handler.repo.Create(order); err != nil {
		// 同時に同じキーでリクエストが来た場合の競合に備え、ユニーク制約違反は
		// 「先に成功した方の結果を返す」形で救済する。
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if existing, findErr := handler.repo.FindByIdempotencyKey(idempotencyKey); findErr == nil {
				writeJSON(w, http.StatusOK, toOrderResponse(existing))
				return
			}
		}
		writeError(w, apperr.Internal(err))
		return
	}

	writeJSON(w, http.StatusCreated, toOrderResponse(order))
}

// ListOrders は GET /api/v1/orders。ログイン中ユーザー自身の注文一覧を返す。
func (handler *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, apperr.Unauthenticated("login is required"))
		return
	}

	orders, err := handler.repo.FindByUser(claims.UserID)
	if err != nil {
		writeError(w, apperr.Internal(err))
		return
	}

	response := make([]orderResponse, 0, len(orders))
	for i := range orders {
		response = append(response, toOrderResponse(&orders[i]))
	}
	writeJSON(w, http.StatusOK, response)
}

// GetOrder は GET /api/v1/orders/{id}。他ユーザーの注文は(存在を推測されないよう)404として扱う。
func (handler *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
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

	order, err := handler.repo.FindByID(uint(id))
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
