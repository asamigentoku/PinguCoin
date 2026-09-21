package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/payment-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/repository"
)

// PaymentServer は proto の `service PaymentService` の実装。
// 決済処理・履歴取得・キャンセル/返金の3機能をこの1サービスにまとめている。
type PaymentServer struct {
	pb.UnimplementedPaymentServiceServer
	payments *repository.PaymentRepository
	refunds  *repository.RefundRepository
}

func NewPaymentServer(payments *repository.PaymentRepository, refunds *repository.RefundRepository) *PaymentServer {
	return &PaymentServer{payments: payments, refunds: refunds}
}

// CreatePayment は決済処理そのもの。
// 現時点では外部決済ゲートウェイとの連携は実装しておらず、その場で成功として処理する
// (ポイント決済など即時確定の決済を想定)。将来カード決済等の非同期ゲートウェイを繋ぐ場合は、
// ここで status を "pending" のまま返し、Webhook等で確定させる形に拡張する。
func (s *PaymentServer) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	if req.GetUserId() == 0 {
		return nil, apperr.InvalidArgument("user_id is required")
	}
	if req.GetProductId() == 0 {
		return nil, apperr.InvalidArgument("product_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}
	if strings.TrimSpace(req.GetCurrency()) == "" {
		return nil, apperr.InvalidArgument("currency is required")
	}
	if strings.TrimSpace(req.GetPaymentMethod()) == "" {
		return nil, apperr.InvalidArgument("payment_method is required")
	}

	payment := &model.Payment{
		UserID:        uint(req.GetUserId()),
		ProductID:     uint(req.GetProductId()),
		Amount:        req.GetAmount(),
		Currency:      req.GetCurrency(),
		PaymentMethod: req.GetPaymentMethod(),
		Status:        model.PaymentStatusSucceeded,
	}

	if err := s.payments.Create(payment); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CreatePaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// GetPayment は決済を1件取得する。
func (s *PaymentServer) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
	payment, err := s.payments.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}
	return &pb.GetPaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// ListPayments は決済履歴を返す。user_id を指定するとそのユーザーの履歴に絞り込む。
func (s *PaymentServer) ListPayments(ctx context.Context, req *pb.ListPaymentsRequest) (*pb.ListPaymentsResponse, error) {
	var userID uint
	if req.UserId != nil {
		userID = uint(req.GetUserId())
	}

	payments, err := s.payments.FindAll(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListPaymentsResponse{}
	for i := range payments {
		resp.Payments = append(resp.Payments, toProtoPayment(&payments[i]))
	}
	return resp, nil
}

// ListRefunds は返金履歴を返す。payment_id を指定するとその決済の返金に絞り込む。
func (s *PaymentServer) ListRefunds(ctx context.Context, req *pb.ListRefundsRequest) (*pb.ListRefundsResponse, error) {
	var paymentID uint
	if req.PaymentId != nil {
		paymentID = uint(req.GetPaymentId())
	}

	refunds, err := s.refunds.FindAll(paymentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListRefundsResponse{}
	for i := range refunds {
		resp.Refunds = append(resp.Refunds, toProtoRefund(&refunds[i]))
	}
	return resp, nil
}

// CancelPayment は決済が確定する前(status=pending)のキャンセル。
// 確定済み決済を取り消したい場合は RefundPayment を使う。
func (s *PaymentServer) CancelPayment(ctx context.Context, req *pb.CancelPaymentRequest) (*pb.CancelPaymentResponse, error) {
	payment, err := s.payments.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}

	if payment.Status != model.PaymentStatusPending {
		return nil, apperr.FailedPrecondition("payment is not cancellable in status: " + payment.Status)
	}

	payment.Status = model.PaymentStatusCanceled
	if err := s.payments.Update(payment); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.CancelPaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// RefundPayment は確定済み決済(succeeded/partially_refunded)に対する返金。
// amount を指定することで部分返金にも対応し、累計返金額が決済額を超える場合は拒否する。
func (s *PaymentServer) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
	if req.GetPaymentId() == 0 {
		return nil, apperr.InvalidArgument("payment_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, apperr.InvalidArgument("amount must be greater than zero")
	}

	payment, err := s.payments.FindByID(uint(req.GetPaymentId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}

	if payment.Status != model.PaymentStatusSucceeded && payment.Status != model.PaymentStatusPartiallyRefunded {
		return nil, apperr.FailedPrecondition("payment is not refundable in status: " + payment.Status)
	}

	alreadyRefunded, err := s.refunds.SumSucceededAmount(payment.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	remaining := payment.Amount - alreadyRefunded
	if req.GetAmount() > remaining {
		return nil, apperr.InvalidArgument("refund amount exceeds refundable amount")
	}

	refund := &model.Refund{
		PaymentID: payment.ID,
		Amount:    req.GetAmount(),
		Reason:    req.GetReason(),
		Status:    model.RefundStatusSucceeded,
	}
	if err := s.refunds.Create(refund); err != nil {
		return nil, apperr.Internal(err)
	}

	if req.GetAmount() == remaining {
		payment.Status = model.PaymentStatusRefunded
	} else {
		payment.Status = model.PaymentStatusPartiallyRefunded
	}
	if err := s.payments.Update(payment); err != nil {
		return nil, apperr.Internal(err)
	}

	return &pb.RefundPaymentResponse{
		Refund:  toProtoRefund(refund),
		Payment: toProtoPayment(payment),
	}, nil
}

func toProtoPayment(p *model.Payment) *pb.Payment {
	return &pb.Payment{
		Id:            uint32(p.ID),
		UserId:        uint32(p.UserID),
		ProductId:     uint32(p.ProductID),
		Amount:        p.Amount,
		Currency:      p.Currency,
		PaymentMethod: p.PaymentMethod,
		Status:        p.Status,
		CreatedAt:     timestamppb.New(p.CreatedAt),
		UpdatedAt:     timestamppb.New(p.UpdatedAt),
	}
}

func toProtoRefund(r *model.Refund) *pb.Refund {
	return &pb.Refund{
		Id:        uint32(r.ID),
		PaymentId: uint32(r.PaymentID),
		Amount:    r.Amount,
		Reason:    r.Reason,
		Status:    r.Status,
		CreatedAt: timestamppb.New(r.CreatedAt),
		UpdatedAt: timestamppb.New(r.UpdatedAt),
	}
}
