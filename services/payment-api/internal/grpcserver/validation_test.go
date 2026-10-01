package grpcserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

func TestMapFindError(t *testing.T) {
	if got := status.Code(mapFindError("payment", gorm.ErrRecordNotFound)); got != codes.NotFound {
		t.Errorf("record-not-found should map to NotFound, got %v", got)
	}

	internal := mapFindError("payment", errors.New("connection reset by peer"))
	if status.Code(internal) != codes.Internal {
		t.Errorf("other errors should map to Internal, got %v", status.Code(internal))
	}
	if status.Convert(internal).Message() != "internal server error" {
		t.Errorf("internal error leaks details: %q", status.Convert(internal).Message())
	}
	if appErr, ok := apperr.As(internal); !ok || appErr.Err == nil {
		t.Error("the original error should be kept for the server log")
	}
}

func TestToProtoPayment(t *testing.T) {
	created := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	got := toProtoPayment(&model.Payment{
		ID: 9, UserID: 3, ProductID: 5, Amount: 1200, Currency: "POINT", PaymentMethod: "point",
		Status: model.PaymentStatusSucceeded, IdempotencyKey: "secret-key", CreatedAt: created, UpdatedAt: created,
	})

	if got.GetId() != 9 || got.GetUserId() != 3 || got.GetProductId() != 5 || got.GetAmount() != 1200 ||
		got.GetCurrency() != "POINT" || got.GetPaymentMethod() != "point" || got.GetStatus() != "succeeded" {
		t.Errorf("unexpected conversion: %+v", got)
	}
	if !got.GetCreatedAt().AsTime().Equal(created) {
		t.Errorf("created_at = %v", got.GetCreatedAt().AsTime())
	}
}

func TestToProtoRefund(t *testing.T) {
	got := toProtoRefund(&model.Refund{ID: 2, PaymentID: 9, Amount: 300, Reason: "duplicate", Status: model.RefundStatusSucceeded})
	if got.GetId() != 2 || got.GetPaymentId() != 9 || got.GetAmount() != 300 || got.GetReason() != "duplicate" || got.GetStatus() != "succeeded" {
		t.Errorf("unexpected conversion: %+v", got)
	}
}

// 入力チェックはDBに触れる前に行われるので、DBが無くても検証できる。
func TestServersRejectInvalidInputBeforeTouchingTheDatabase(t *testing.T) {
	ctx := context.Background()
	payments := NewPaymentServer(nil, nil, nil, nil)
	points := NewPointServer(nil)

	valid := func() *pb.CreatePaymentRequest {
		return &pb.CreatePaymentRequest{UserId: 1, ProductId: 2, Amount: 100, Currency: "POINT", PaymentMethod: "point", IdempotencyKey: "k"}
	}
	tests := []struct {
		name string
		call func() error
	}{
		{"payment without user", func() error {
			r := valid()
			r.UserId = 0
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment without product", func() error {
			r := valid()
			r.ProductId = 0
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment of zero", func() error {
			r := valid()
			r.Amount = 0
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment of a negative amount", func() error {
			r := valid()
			r.Amount = -100
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment without currency", func() error {
			r := valid()
			r.Currency = "  "
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment without method", func() error {
			r := valid()
			r.PaymentMethod = ""
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"payment without idempotency key", func() error {
			r := valid()
			r.IdempotencyKey = ""
			_, err := payments.CreatePayment(ctx, r)
			return err
		}},
		{"refund without payment", func() error {
			_, err := payments.RefundPayment(ctx, &pb.RefundPaymentRequest{Amount: 10})
			return err
		}},
		{"refund of zero", func() error {
			_, err := payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: 1})
			return err
		}},
		{"get points without user", func() error {
			_, err := points.GetPointAccount(ctx, &pb.GetPointAccountRequest{})
			return err
		}},
		{"credit without user", func() error {
			_, err := points.CreditPoints(ctx, &pb.CreditPointsRequest{Amount: 10, Reason: "campaign"})
			return err
		}},
		{"credit of zero", func() error {
			_, err := points.CreditPoints(ctx, &pb.CreditPointsRequest{UserId: 1, Reason: "campaign"})
			return err
		}},
		{"credit without reason", func() error {
			_, err := points.CreditPoints(ctx, &pb.CreditPointsRequest{UserId: 1, Amount: 10, Reason: "  "})
			return err
		}},
		{"debit without user", func() error {
			_, err := points.DebitPoints(ctx, &pb.DebitPointsRequest{Amount: 10, Reason: "adjust"})
			return err
		}},
		{"debit of a negative amount", func() error {
			_, err := points.DebitPoints(ctx, &pb.DebitPointsRequest{UserId: 1, Amount: -5, Reason: "adjust"})
			return err
		}},
		{"debit without reason", func() error {
			_, err := points.DebitPoints(ctx, &pb.DebitPointsRequest{UserId: 1, Amount: 5})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status.Code(tt.call()); got != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", got)
			}
		})
	}
}
