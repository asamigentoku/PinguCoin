package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/application"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/paymentmethod"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
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
	create   *application.PaymentUseCase
}

func NewPaymentServer(db *gorm.DB, payments *repository.PaymentRepository, refunds *repository.RefundRepository, points *repository.PointRepository) *PaymentServer {
	return NewPaymentServerWithUseCase(db, payments, refunds, points, application.NewPaymentUseCase(db, paymentmethod.InMemoryStripe{}))
}

func NewPaymentServerWithUseCase(db *gorm.DB, payments *repository.PaymentRepository, refunds *repository.RefundRepository, points *repository.PointRepository, create *application.PaymentUseCase) *PaymentServer {
	return &PaymentServer{db: db, payments: payments, refunds: refunds, points: points, create: create}
}

// CreatePayment は決済処理そのもの。
// pointはDB内で即時確定し、card/credit_cardはPaymentUseCaseからmock Stripeを呼び出す。
func (server *PaymentServer) CreatePayment(ctx context.Context, request *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	replayed := false
	response, err := server.createPayment(ctx, request, &replayed)

	// メトリクス: 決済の数を、結果(succeeded / replayed / insufficient / rejected / failed)別に、1回だけ数える。
	// 支払い方法と通貨は、クライアントが決められる文字列なので、ラベルには、既知の値だけを使う(それ以外は other)。
	result := metrics.ResultOf(err)
	switch {
	case err == nil && replayed:
		result = metrics.ResultReplayed
	case status.Code(err) == codes.FailedPrecondition:
		result = metrics.ResultInsufficient // この RPC の FailedPrecondition は、ポイント不足だけ
	}
	var amount int64
	if err == nil && !replayed {
		amount = request.GetAmount()
	}
	metrics.RecordPayment(
		metrics.Label(request.GetPaymentMethod(), model.PaymentMethodPoint),
		metrics.Label(request.GetCurrency(), "POINT"),
		result, amount)
	return response, err
}

// createPayment は CreatePayment の本体。replayed には、同じ冪等性キーの再送で、最初の結果を返したかどうかを書き込む(メトリクス用)。
func (server *PaymentServer) createPayment(ctx context.Context, request *pb.CreatePaymentRequest, replayed *bool) (*pb.CreatePaymentResponse, error) {
	if server.create != nil {
		result, err := server.create.Create(ctx, application.CreatePaymentCommand{UserID: uint(request.GetUserId()), ProductID: uint(request.GetProductId()), Amount: request.GetAmount(), Currency: request.GetCurrency(), PaymentMethod: request.GetPaymentMethod(), IdempotencyKey: request.GetIdempotencyKey(), Purpose: request.GetPurpose()})
		if err != nil {
			return nil, apperr.Internal(err)
		}
		*replayed = result.Replayed
		if result.Rejected != nil {
			switch {
			case errors.Is(result.Rejected, application.ErrInvalid), errors.Is(result.Rejected, application.ErrUnsupportedMethod):
				return nil, apperr.InvalidArgument(result.Rejected.Error())
			case errors.Is(result.Rejected, application.ErrInsufficientPoints):
				return nil, apperr.FailedPrecondition(result.Rejected.Error())
			default:
				return nil, apperr.FailedPrecondition(result.Rejected.Error())
			}
		}
		response := toProtoPayment(result.Payment)
		response.ActionUrl = result.ActionURL
		return &pb.CreatePaymentResponse{Payment: response}, nil
	}
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
	idempotencyKey := strings.TrimSpace(request.GetIdempotencyKey())
	if idempotencyKey == "" {
		return nil, apperr.InvalidArgument("idempotency_key is required")
	}
	db := server.db.WithContext(ctx)
	payments := server.payments.WithTx(db)

	// 同じidempotency_keyの決済が既にあれば、新たに決済(ポイント消費含む)を作らず
	// そのまま返す(リトライ・二重送信で二重決済にならないようにするため)。
	if existing, err := payments.FindByIdempotencyKey(idempotencyKey); err == nil {
		*replayed = true
		return &pb.CreatePaymentResponse{Payment: toProtoPayment(existing)}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.Internal(err)
	}

	payment := &model.Payment{
		UserID:         uint(request.GetUserId()),
		ProductID:      uint(request.GetProductId()),
		Amount:         request.GetAmount(),
		Currency:       request.GetCurrency(),
		PaymentMethod:  request.GetPaymentMethod(),
		Status:         model.PaymentStatusSucceeded,
		IdempotencyKey: idempotencyKey,
	}

	// point払いの場合は「決済の作成」と「ポイント残高の消費」を1トランザクションにまとめる。
	// 片方だけ成功する(決済は作られたのにポイントは減っていない、等)状態を防ぐため。
	if payment.PaymentMethod == model.PaymentMethodPoint {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := payments.WithTx(tx).Create(payment); err != nil {
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
			if existing, ok := server.recoverDuplicatePayment(ctx, err, idempotencyKey); ok {
				*replayed = true
				return &pb.CreatePaymentResponse{Payment: toProtoPayment(existing)}, nil
			}
			return nil, apperr.Internal(err)
		}
		return &pb.CreatePaymentResponse{Payment: toProtoPayment(payment)}, nil
	}

	if err := payments.Create(payment); err != nil {
		if existing, ok := server.recoverDuplicatePayment(ctx, err, idempotencyKey); ok {
			*replayed = true
			return &pb.CreatePaymentResponse{Payment: toProtoPayment(existing)}, nil
		}
		return nil, apperr.Internal(err)
	}
	return &pb.CreatePaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// recoverDuplicatePayment は、同時に同じidempotency_keyでリクエストが来た場合の競合に備え、
// ユニーク制約違反(gorm.ErrDuplicatedKey)を「先に成功した方の結果を返す」形で救済する。
func (server *PaymentServer) recoverDuplicatePayment(ctx context.Context, err error, idempotencyKey string) (*model.Payment, bool) {
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil, false
	}
	existing, findErr := server.payments.WithTx(server.db.WithContext(ctx)).FindByIdempotencyKey(idempotencyKey)
	if findErr != nil {
		return nil, false
	}
	return existing, true
}

// GetPayment は決済を1件取得する。
func (server *PaymentServer) GetPayment(ctx context.Context, request *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
	payment, err := server.payments.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("payment", err)
	}
	response := toProtoPayment(payment)
	if source, sourceErr := repository.NewPaymentSourceRepository(server.db.WithContext(ctx)).FindByPaymentID(payment.ID); sourceErr == nil {
		response.ActionUrl = source.ActionURL
	}
	return &pb.GetPaymentResponse{Payment: response}, nil
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

	if payment.Status != model.PaymentStatusPending && payment.Status != model.PaymentStatusProcessing {
		return nil, apperr.FailedPrecondition("payment is not cancellable in status: " + payment.Status)
	}

	if payment.PaymentMethod != model.PaymentMethodPoint {
		if err := server.create.CancelExternal(ctx, payment); err != nil {
			return nil, apperr.Internal(err)
		}
	} else {
		payment.Status = model.PaymentStatusCanceled
	}
	if payment.PaymentMethod == model.PaymentMethodPoint {
		if updateErr := server.payments.Update(payment); updateErr != nil {
			return nil, apperr.Internal(updateErr)
		}
	}
	return &pb.CancelPaymentResponse{Payment: toProtoPayment(payment)}, nil
}

// RefundPayment は確定済み決済(succeeded/partially_refunded)に対する返金。
// amount を指定することで部分返金にも対応し、累計返金額が決済額を超える場合は拒否する。
func (server *PaymentServer) RefundPayment(ctx context.Context, request *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
	response, err := server.refundPayment(ctx, request)
	metrics.RecordRefund(metrics.ResultOf(err)) // メトリクス: 返金の数(結果別)
	return response, err
}

// refundPayment は RefundPayment の本体。
func (server *PaymentServer) refundPayment(ctx context.Context, request *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
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
	if payment.PaymentMethod != model.PaymentMethodPoint {
		key := fmt.Sprintf("refund-%d-%d", payment.ID, alreadyRefunded+request.GetAmount())
		if err := server.create.RefundExternal(ctx, payment, request.GetAmount(), key); err != nil {
			return nil, apperr.Internal(err)
		}
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

	if err := server.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := server.refunds.WithTx(tx).Create(refund); err != nil {
			return err
		}
		payment.Status = newStatus
		return server.payments.WithTx(tx).Update(payment)
	}); err != nil {
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
		Purpose:       payment.Purpose,
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
