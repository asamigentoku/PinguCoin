package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/payment-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/repository"
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
func (s *PointServer) GetPointAccount(ctx context.Context, req *pb.GetPointAccountRequest) (*pb.GetPointAccountResponse, error) {
	if req.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}

	account, err := s.points.GetAccount(uint(req.GetUserId()))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		account = &model.PointAccount{UserID: uint(req.GetUserId()), Balance: 0}
	} else if err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.GetPointAccountResponse{Account: toProtoPointAccount(account)}, nil
}

// ListPointTransactions はポイント増減履歴を返す。user_id を指定するとそのユーザーに絞り込む。
func (s *PointServer) ListPointTransactions(ctx context.Context, req *pb.ListPointTransactionsRequest) (*pb.ListPointTransactionsResponse, error) {
	var userID uint
	if req.UserId != nil {
		userID = uint(req.GetUserId())
	}

	transactions, err := s.points.ListTransactions(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListPointTransactionsResponse{}
	for i := range transactions {
		resp.Transactions = append(resp.Transactions, toProtoPointTransaction(&transactions[i]))
	}
	return resp, nil
}

// CreditPoints は決済連携以外の手動付与(例: キャンペーン)。
func (s *PointServer) CreditPoints(ctx context.Context, req *pb.CreditPointsRequest) (*pb.CreditPointsResponse, error) {
	if req.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(req.GetReason()) == "" {
		return nil, apperr.InvalidArgument("reason is required")
	}

	transaction, err := s.points.AdjustAtomic(uint(req.GetUserId()), req.GetAmount(), model.PointTransactionTypeCredit, req.GetReason(), nil)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.CreditPointsResponse{
		Transaction: toProtoPointTransaction(transaction),
		Account:     &pb.PointAccount{UserId: req.GetUserId(), Balance: transaction.BalanceAfter},
	}, nil
}

// DebitPoints は決済連携以外の手動消費(例: 運用による調整)。
func (s *PointServer) DebitPoints(ctx context.Context, req *pb.DebitPointsRequest) (*pb.DebitPointsResponse, error) {
	if req.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(req.GetReason()) == "" {
		return nil, apperr.InvalidArgument("reason is required")
	}

	transaction, err := s.points.AdjustAtomic(uint(req.GetUserId()), -req.GetAmount(), model.PointTransactionTypeDebit, req.GetReason(), nil)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientPoints) {
			return nil, apperr.FailedPrecondition("insufficient points")
		}
		return nil, apperr.Internal(err)
	}

	return &pb.DebitPointsResponse{
		Transaction: toProtoPointTransaction(transaction),
		Account:     &pb.PointAccount{UserId: req.GetUserId(), Balance: transaction.BalanceAfter},
	}, nil
}

func toProtoPointAccount(a *model.PointAccount) *pb.PointAccount {
	p := &pb.PointAccount{
		UserId:  uint32(a.UserID),
		Balance: a.Balance,
	}
	if !a.UpdatedAt.IsZero() {
		p.UpdatedAt = timestamppb.New(a.UpdatedAt)
	}
	return p
}

func toProtoPointTransaction(t *model.PointTransaction) *pb.PointTransaction {
	p := &pb.PointTransaction{
		Id:           uint32(t.ID),
		UserId:       uint32(t.UserID),
		Amount:       t.Amount,
		Type:         t.Type,
		Reason:       t.Reason,
		BalanceAfter: t.BalanceAfter,
		CreatedAt:    timestamppb.New(t.CreatedAt),
	}
	if t.PaymentID != nil {
		v := uint32(*t.PaymentID)
		p.PaymentId = &v
	}
	return p
}
