package grpcserver

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/payment-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/repository"
)

// PaymentServer は proto の `service PaymentService` の実装。
// 決済処理・履歴取得・キャンセル/返金の3機能をこの1サービスにまとめている。
// payment_method="point" の場合、db.Transaction で決済とポイント増減(internal/repository.PointRepository)を
// 1つのトランザクションにまとめて原子的に処理する(片方だけ成功する状態を作らないため)。
type PaymentServer struct {
	pb.UnimplementedPaymentServiceServer
	db       *gorm.DB
	payments *repository.PaymentRepository
	refunds  *repository.RefundRepository
	points   *repository.PointRepository
}

func NewPaymentServer(db *gorm.DB, payments *repository.PaymentRepository, refunds *repository.RefundRepository, points *repository.PointRepository) *PaymentServer {
	return &PaymentServer{db: db, payments: payments, refunds: refunds, points: points}
}

// CreatePayment は決済処理そのもの。
// 現時点では外部決済ゲートウェイとの連携は実装しておらず、その場で成功として処理する
// (ポイント決済など即時確定の決済を想定)。将来カード決済等の非同期ゲートウェイを繋ぐ場合は、
// ここで status を "pending" のまま返し、Webhook等で確定させる形に拡張する。
func (server *PaymentServer) CreatePayment(ctx context.Context, request *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	if request.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if request.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if request.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(request.GetCurrency()) == "" {
		return nil, apperr.InvalidArgument("currency is required")
	}
	if strings.TrimSpace(request.GetPaymentMethod()) == "" {
		return nil, apperr.InvalidArgument("payment_method is required")
	}

	payment := &model.Payment{
		UserID:        uint(request.GetUserId()),
		ProductID:     uint(request.GetProductId()),
		Amount:        request.GetAmount(),
		Currency:      request.GetCurrency(),
		PaymentMethod: request.GetPaymentMethod(),
		Status:        model.PaymentStatusSucceeded,
	}

	// point払いの場合は「決済の作成」と「ポイント残高の消費」を1トランザクションにまとめる。
	// 片方だけ成功する(決済は作られたのにポイントは減っていない、等)状態を防ぐため。
	if payment.PaymentMethod == model.PaymentMethodPoint {
		err := server.db.Transaction(func(tx *gorm.DB) error {
			if err := server.payments.WithTx(tx).Create(payment); err != nil {
				return err
			}
			paymentID := payment.ID
			_, err := server.points.WithTx(tx).Adjust(payment.UserID, -payment.Amount, model.PointTransactionTypePayment, "payment #"+strconv.FormatUint(uint64(payment.ID), 10), &paymentID)
			return err
		})
		if err != nil {
			if errors.Is(err, repository.ErrInsufficientPoints) {
				return nil, apperr.FailedPrecondition("insufficient points")
			}
			return nil, apperr.Internal(err)
		}
		return &pb.CreatePaymentResponse{Payment: toProtoPayment(payment)}, nil
	}

	if err := server.payments.Create(payment); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreatePaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// GetPayment は決済を1件取得する。
func (server *PaymentServer) GetPayment(ctx context.Context, request *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
	payment, err := server.payments.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}
	return &pb.GetPaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// ListPayments は決済履歴を返す。user_id を指定するとそのユーザーの履歴に絞り込む。
func (server *PaymentServer) ListPayments(ctx context.Context, request *pb.ListPaymentsRequest) (*pb.ListPaymentsResponse, error) {
	var userID uint
	if request.UserId != nil {
		userID = uint(request.GetUserId())
	}

	payments, err := server.payments.FindAll(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListPaymentsResponse{}
	for i := range payments {
		response.Payments = append(response.Payments, toProtoPayment(&payments[i]))
	}
	return response, nil
}

// ListRefunds は返金履歴を返す。payment_id を指定するとその決済の返金に絞り込む。
func (server *PaymentServer) ListRefunds(ctx context.Context, request *pb.ListRefundsRequest) (*pb.ListRefundsResponse, error) {
	var paymentID uint
	if request.PaymentId != nil {
		paymentID = uint(request.GetPaymentId())
	}

	refunds, err := server.refunds.FindAll(paymentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListRefundsResponse{}
	for i := range refunds {
		response.Refunds = append(response.Refunds, toProtoRefund(&refunds[i]))
	}
	return response, nil
}

// CancelPayment は決済が確定する前(status=pending)のキャンセル。
// 確定済み決済を取り消したい場合は RefundPayment を使う。
func (server *PaymentServer) CancelPayment(ctx context.Context, request *pb.CancelPaymentRequest) (*pb.CancelPaymentResponse, error) {
	payment, err := server.payments.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}

	if payment.Status != model.PaymentStatusPending {
		return nil, apperr.FailedPrecondition("payment is not cancellable in status: " + payment.Status)
	}

	payment.Status = model.PaymentStatusCanceled
	if err := server.payments.Update(payment); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CancelPaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// RefundPayment は確定済み決済(succeeded/partially_refunded)に対する返金。
// amount を指定することで部分返金にも対応し、累計返金額が決済額を超える場合は拒否する。
func (server *PaymentServer) RefundPayment(ctx context.Context, request *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
	if request.GetPaymentId() == 0 {
		return nil, apperr.InvalidArgument("payment_id is required")
	}
	if request.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}

	payment, err := server.payments.FindByID(uint(request.GetPaymentId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}

	if payment.Status != model.PaymentStatusSucceeded && payment.Status != model.PaymentStatusPartiallyRefunded {
		return nil, apperr.FailedPrecondition("payment is not refundable in status: " + payment.Status)
	}

	alreadyRefunded, err := server.refunds.SumSucceededAmount(payment.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	remaining := payment.Amount - alreadyRefunded
	if request.GetAmount() > remaining {
		return nil, apperr.InvalidArgument("refund amount exceeds refundable amount")
	}

	refund := &model.Refund{
		PaymentID: payment.ID,
		Amount:    request.GetAmount(),
		Reason:    request.GetReason(),
		Status:    model.RefundStatusSucceeded,
	}

	newStatus := model.PaymentStatusPartiallyRefunded
	if request.GetAmount() == remaining {
		newStatus = model.PaymentStatusRefunded
	}

	// point払いの決済を返金する場合は、返金の記録・決済ステータス更新・ポイント払い戻しを
	// 1トランザクションにまとめる(CreatePaymentと同じ考え方)。
	if payment.PaymentMethod == model.PaymentMethodPoint {
		err := server.db.Transaction(func(tx *gorm.DB) error {
			if err := server.refunds.WithTx(tx).Create(refund); err != nil {
				return err
			}
			payment.Status = newStatus
			if err := server.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			paymentID := payment.ID
			_, err := server.points.WithTx(tx).Adjust(payment.UserID, refund.Amount, model.PointTransactionTypeRefund, "refund #"+strconv.FormatUint(uint64(refund.ID), 10), &paymentID)
			return err
		})
		if err != nil {
			return nil, apperr.Internal(err)
		}
		return &pb.RefundPaymentResponse{Refund: toProtoRefund(refund), Payment: toProtoPayment(payment)}, nil
	}

	if err := server.refunds.Create(refund); err != nil {
		return nil, apperr.Internal(err)
	}

	payment.Status = newStatus
	if err := server.payments.Update(payment); err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.RefundPaymentResponse{
		Refund:  toProtoRefund(refund),
		Payment: toProtoPayment(payment),
	}, nil
}

func toProtoPayment(payment *model.Payment) *pb.Payment {
	return &pb.Payment{
		Id:            uint32(payment.ID),
		UserId:        uint32(payment.UserID),
		ProductId:     uint32(payment.ProductID),
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		PaymentMethod: payment.PaymentMethod,
		Status:        payment.Status,
		CreatedAt:     timestamppb.New(payment.CreatedAt),
		UpdatedAt:     timestamppb.New(payment.UpdatedAt),
	}
}

func toProtoRefund(refund *model.Refund) *pb.Refund {
	return &pb.Refund{
		Id:        uint32(refund.ID),
		PaymentId: uint32(refund.PaymentID),
		Amount:    refund.Amount,
		Reason:    refund.Reason,
		Status:    refund.Status,
		CreatedAt: timestamppb.New(refund.CreatedAt),
		UpdatedAt: timestamppb.New(refund.UpdatedAt),
	}
}
