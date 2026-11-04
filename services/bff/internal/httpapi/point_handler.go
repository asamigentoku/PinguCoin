package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/asamigentoku/PinguCoin/services/bff/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/reqcontext"
	paymentpb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

// welcomeBonusPoints は初めてポイントを参照したユーザーに1度だけ付与する初期ポイント。
const welcomeBonusPoints = 1600

// PointHandler はログイン中ユーザー自身のポイント残高・履歴を返す。
type PointHandler struct {
	payment *paymentclient.Client
}

type paymentResponse struct {
	ID        uint32 `json:"id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
	Purpose   string `json:"purpose"`
	ActionURL string `json:"action_url,omitempty"`
}

func pointPaymentResponse(payment *paymentpb.Payment) paymentResponse {
	return paymentResponse{ID: payment.GetId(), Amount: payment.GetAmount(), Currency: payment.GetCurrency(), Status: payment.GetStatus(), Purpose: payment.GetPurpose(), ActionURL: payment.GetActionUrl()}
}

// TopUpPoints starts an external payment. Points are credited only by the
// signed provider webhook, never by this request response.
func (handler *PointHandler) TopUpPoints(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		Amount        int64  `json:"amount"`
		PaymentMethod string `json:"payment_method"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.Amount < 100 || body.Amount > 100000 {
		writeError(w, r, apperr.InvalidArgument("amount must be between 100 and 100000"))
		return
	}
	if body.PaymentMethod != "card" && body.PaymentMethod != "credit_card" && body.PaymentMethod != "bank_transfer" {
		writeError(w, r, apperr.InvalidArgument("unsupported top-up payment_method"))
		return
	}
	response, err := handler.payment.Payment.CreatePayment(r.Context(), &paymentpb.CreatePaymentRequest{UserId: uint32(claims.UserID), Amount: body.Amount, Currency: "JPY", PaymentMethod: body.PaymentMethod, IdempotencyKey: "topup:" + key, Purpose: "point_topup"})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusAccepted, pointPaymentResponse(response.GetPayment()))
}

func (handler *PointHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthenticated("login is required"))
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil {
		writeError(w, r, apperr.InvalidArgument("invalid payment id"))
		return
	}
	response, err := handler.payment.Payment.GetPayment(r.Context(), &paymentpb.GetPaymentRequest{Id: uint32(id)})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	if response.GetPayment().GetUserId() != uint32(claims.UserID) {
		writeError(w, r, apperr.NotFound("payment"))
		return
	}
	writeJSON(w, http.StatusOK, pointPaymentResponse(response.GetPayment()))
}

func NewPointHandler(payment *paymentclient.Client) *PointHandler {
	return &PointHandler{payment: payment}
}

type pointTransactionResponse struct {
	ID              uint32 `json:"id"`
	Amount          int64  `json:"amount"`
	Type            string `json:"type"`
	Reason          string `json:"reason"`
	BalanceAfter    int64  `json:"balance_after"`
	CreatedAt       string `json:"created_at"`
	ChainSequence   uint64 `json:"chain_sequence"`
	PreviousHash    string `json:"previous_hash"`
	TransactionHash string `json:"transaction_hash"`
}

type pointsResponse struct {
	Balance      int64                      `json:"balance"`
	Transactions []pointTransactionResponse `json:"transactions"`
}

// GetPoints は GET /api/v1/points。残高と履歴(新しい順)を返す。
// 取引が1件も無いユーザー(初回)には、ウェルカムボーナスを1度だけ付与する。
func (handler *PointHandler) GetPoints(w http.ResponseWriter, r *http.Request) {
	claims, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthenticated("login is required"))
		return
	}
	userID := uint32(claims.UserID)

	list, err := handler.payment.Point.ListPointTransactions(r.Context(), &paymentpb.ListPointTransactionsRequest{UserId: &userID})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	if len(list.GetTransactions()) == 0 {
		if _, err := handler.payment.Point.CreditPoints(r.Context(), &paymentpb.CreditPointsRequest{UserId: userID, Amount: welcomeBonusPoints, Reason: "ウェルカムボーナス"}); err != nil {
			writeError(w, r, apperr.FromGRPC(err))
			return
		}
		if list, err = handler.payment.Point.ListPointTransactions(r.Context(), &paymentpb.ListPointTransactionsRequest{UserId: &userID}); err != nil {
			writeError(w, r, apperr.FromGRPC(err))
			return
		}
	}

	account, err := handler.payment.Point.GetPointAccount(r.Context(), &paymentpb.GetPointAccountRequest{UserId: userID})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}

	response := pointsResponse{Balance: account.GetAccount().GetBalance(), Transactions: make([]pointTransactionResponse, 0, len(list.GetTransactions()))}
	for _, transaction := range list.GetTransactions() {
		response.Transactions = append(response.Transactions, pointTransactionResponse{
			ID: transaction.GetId(), Amount: transaction.GetAmount(), Type: transaction.GetType(), Reason: transaction.GetReason(),
			BalanceAfter: transaction.GetBalanceAfter(), CreatedAt: transaction.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z07:00"),
			ChainSequence: transaction.GetChainSequence(), PreviousHash: transaction.GetPreviousHash(), TransactionHash: transaction.GetTransactionHash(),
		})
	}
	// 履歴は payment-api が新しい順(id降順)で返すので、並べ替えずにそのまま返す。
	writeJSON(w, http.StatusOK, response)
}
