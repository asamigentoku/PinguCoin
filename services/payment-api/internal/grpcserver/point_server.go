package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

// PointServer は proto の `service PointService` の実装。
// ユーザーごとのポイント残高・増減履歴の参照と、決済連携以外の手動付与/消費を扱う。
// point払いの決済に伴う自動増減は PaymentServer(payment_server.go)側で行う。
type PointServer struct {
	pb.UnimplementedPointServiceServer
	points *repository.PointRepository
}

func NewPointServer(points *repository.PointRepository) *PointServer {
	return &PointServer{points: points}
}

// GetPointAccount は残高を取得する。一度も取引がないユーザーはレコードが存在しないため、
// その場合はエラーにせず残高0の口座として返す。
func (server *PointServer) GetPointAccount(ctx context.Context, request *pb.GetPointAccountRequest) (*pb.GetPointAccountResponse, error) {
	if request.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}

	account, err := server.points.GetAccount(uint(request.GetUserId()))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		account = &model.PointAccount{UserID: uint(request.GetUserId()), Balance: 0}
	} else if err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.GetPointAccountResponse{Account: toProtoPointAccount(account)}, nil
}

// ListPointTransactions はポイント増減履歴を返す。user_id を指定するとそのユーザーに絞り込む。
func (server *PointServer) ListPointTransactions(ctx context.Context, request *pb.ListPointTransactionsRequest) (*pb.ListPointTransactionsResponse, error) {
	var userID uint
	if request.UserId != nil {
		userID = uint(request.GetUserId())
	}

	transactions, err := server.points.ListTransactions(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListPointTransactionsResponse{}
	for i := range transactions {
		response.Transactions = append(response.Transactions, toProtoPointTransaction(&transactions[i]))
	}
	return response, nil
}

// CreditPoints は決済連携以外の手動付与(例: キャンペーン)。
func (server *PointServer) CreditPoints(ctx context.Context, request *pb.CreditPointsRequest) (*pb.CreditPointsResponse, error) {
	response, err := server.creditPoints(ctx, request)
	metrics.RecordPointTransaction(model.PointTransactionTypeCredit, metrics.ResultOf(err), request.GetAmount()) // メトリクス: ポイントの付与
	return response, err
}

// creditPoints は CreditPoints の本体。
func (server *PointServer) creditPoints(ctx context.Context, request *pb.CreditPointsRequest) (*pb.CreditPointsResponse, error) {
	if request.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if request.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(request.GetReason()) == "" {
		return nil, apperr.InvalidArgument("reason is required")
	}

	transaction, err := server.points.AdjustAtomic(uint(request.GetUserId()), request.GetAmount(), model.PointTransactionTypeCredit, request.GetReason(), nil)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.CreditPointsResponse{
		Transaction: toProtoPointTransaction(transaction),
		Account:     &pb.PointAccount{UserId: request.GetUserId(), Balance: transaction.BalanceAfter},
	}, nil
}

// DebitPoints は決済連携以外の手動消費(例: 運用による調整)。
func (server *PointServer) DebitPoints(ctx context.Context, request *pb.DebitPointsRequest) (*pb.DebitPointsResponse, error) {
	response, err := server.debitPoints(ctx, request)
	metrics.RecordPointTransaction(model.PointTransactionTypeDebit, metrics.ResultOf(err), request.GetAmount()) // メトリクス: ポイントの消費
	return response, err
}

// debitPoints は DebitPoints の本体。
func (server *PointServer) debitPoints(ctx context.Context, request *pb.DebitPointsRequest) (*pb.DebitPointsResponse, error) {
	if request.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if request.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(request.GetReason()) == "" {
		return nil, apperr.InvalidArgument("reason is required")
	}

	transaction, err := server.points.AdjustAtomic(uint(request.GetUserId()), -request.GetAmount(), model.PointTransactionTypeDebit, request.GetReason(), nil)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientPoints) {
			return nil, apperr.FailedPrecondition("insufficient points")
		}
		return nil, apperr.Internal(err)
	}

	return &pb.DebitPointsResponse{
		Transaction: toProtoPointTransaction(transaction),
		Account:     &pb.PointAccount{UserId: request.GetUserId(), Balance: transaction.BalanceAfter},
	}, nil
}

func toProtoPointAccount(account *model.PointAccount) *pb.PointAccount {
	protoAccount := &pb.PointAccount{
		UserId:  uint32(account.UserID),
		Balance: account.Balance,
	}
	if !account.UpdatedAt.IsZero() {
		protoAccount.UpdatedAt = timestamppb.New(account.UpdatedAt)
	}
	return protoAccount
}

func toProtoPointTransaction(transaction *model.PointTransaction) *pb.PointTransaction {
	protoTransaction := &pb.PointTransaction{
		Id:           uint32(transaction.ID),
		UserId:       uint32(transaction.UserID),
		Amount:       transaction.Amount,
		Type:         transaction.Type,
		Reason:       transaction.Reason,
		BalanceAfter: transaction.BalanceAfter,
		CreatedAt:    timestamppb.New(transaction.CreatedAt),
	}
	if transaction.PaymentID != nil {
		paymentID := uint32(*transaction.PaymentID)
		protoTransaction.PaymentId = &paymentID
	}
	return protoTransaction
}
