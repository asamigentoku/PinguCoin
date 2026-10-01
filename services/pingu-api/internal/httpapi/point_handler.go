package httpapi

import (
	"net/http"

	paymentpb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/reqcontext"
)

// welcomeBonusPoints は初めてポイントを参照したユーザーに1度だけ付与する初期ポイント。
const welcomeBonusPoints = 1600

// PointHandler はログイン中ユーザー自身のポイント残高・履歴を返す。
type PointHandler struct {
	payment *paymentclient.Client
}

func NewPointHandler(payment *paymentclient.Client) *PointHandler {
	return &PointHandler{payment: payment}
}

type pointTransactionResponse struct {
	ID           uint32 `json:"id"`
	Amount       int64  `json:"amount"`
	Type         string `json:"type"`
	Reason       string `json:"reason"`
	BalanceAfter int64  `json:"balance_after"`
	CreatedAt    string `json:"created_at"`
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
		})
	}
	// 履歴は payment-api が新しい順(id降順)で返すので、並べ替えずにそのまま返す。
	writeJSON(w, http.StatusOK, response)
}
