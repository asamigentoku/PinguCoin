package application

import (
	"context"
	"errors"
	"testing"

	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/paymentmethod"
)

type fakeStripe struct {
	calls  int
	err    error
	result paymentmethod.ChargeResult
}

func (f *fakeStripe) Charge(_ context.Context, request paymentmethod.ChargeRequest) (paymentmethod.ChargeResult, error) {
	f.calls++
	if f.err != nil {
		return paymentmethod.ChargeResult{}, f.err
	}
	if f.result.ID != "" {
		return f.result, nil
	}
	return paymentmethod.ChargeResult{ID: "pi_" + request.IdempotencyKey, Status: "succeeded"}, nil
}

func TestAsyncStripeWebhookCreditsTopUpExactlyOnce(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)
	stripe := &fakeStripe{result: paymentmethod.ChargeResult{ID: "pi_3ds", Status: "requires_action", ActionURL: "https://stripe.test/3ds"}}
	usecase := NewPaymentUseCase(db, stripe)
	result, err := usecase.Create(context.Background(), CreatePaymentCommand{UserID: 1, Amount: 1200, Currency: "JPY", PaymentMethod: model.PaymentMethodCard, Purpose: model.PaymentPurposePointTopUp, IdempotencyKey: "topup-1"})
	if err != nil || result.Payment.Status != model.PaymentStatusProcessing || result.ActionURL == "" {
		t.Fatalf("create result=%+v err=%v", result, err)
	}
	event := ProviderEvent{ID: "evt_1", Type: "payment_intent.succeeded", Data: paymentmethod.ChargeResult{ID: "pi_3ds", Status: "succeeded"}}
	if err := usecase.HandleProviderEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := usecase.HandleProviderEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	account, err := usecase.points.GetAccount(1)
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 1200 {
		t.Fatalf("balance=%d, want 1200", account.Balance)
	}
	var count int64
	if err := db.Model(&model.ProviderEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("provider event count=%d, want 1", count)
	}
}

func TestAsyncOrderPublishesActionRequired(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)
	usecase := NewPaymentUseCase(db, &fakeStripe{result: paymentmethod.ChargeResult{ID: "pi_order", Status: "requires_action", ActionURL: "https://stripe.test/3ds"}})
	result, err := usecase.Create(context.Background(), cardCommand())
	if err != nil || result.ActionURL == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var event model.OutboxEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Topic != orderflow.TopicPaymentActionRequired {
		t.Fatalf("topic=%s", event.Topic)
	}
}
func (*fakeStripe) Cancel(context.Context, string) error { return nil }
func (*fakeStripe) Refund(_ context.Context, request paymentmethod.RefundRequest) (paymentmethod.RefundResult, error) {
	return paymentmethod.RefundResult{ID: "re_" + request.IdempotencyKey, Status: "succeeded"}, nil
}

func cardCommand() CreatePaymentCommand {
	order := &orderflow.OrderCreated{OrderID: 7, UserID: 1, ProductID: 2, Quantity: 1, UnitPrice: 500, TotalAmount: 500, PaymentMethod: model.PaymentMethodCreditCard, IdempotencyKey: "order-7"}
	return CreatePaymentCommand{UserID: 1, ProductID: 2, Amount: 500, Currency: "JPY", PaymentMethod: model.PaymentMethodCreditCard, IdempotencyKey: "order-7", Order: order}
}

func TestStripePaymentStoresSourceAndOutboxAtomically(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)
	stripe := &fakeStripe{}
	usecase := NewPaymentUseCase(db, stripe)

	first, err := usecase.Create(context.Background(), cardCommand())
	if err != nil || first.Rejected != nil {
		t.Fatalf("create: result=%+v err=%v", first, err)
	}
	second, err := usecase.Create(context.Background(), cardCommand())
	if err != nil || !second.Replayed {
		t.Fatalf("replay: result=%+v err=%v", second, err)
	}
	if stripe.calls != 1 {
		t.Fatalf("Stripe calls = %d, want 1", stripe.calls)
	}
	var source model.PaymentSource
	if err := db.Where("payment_id = ?", first.Payment.ID).First(&source).Error; err != nil {
		t.Fatal(err)
	}
	if source.Status != model.PaymentSourceStatusCaptured || source.ProviderReference != "pi_order-7" {
		t.Errorf("unexpected source: %+v", source)
	}
	var events []model.OutboxEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Topic != orderflow.TopicPaymentSucceeded || events[0].PublishedAt != nil {
		t.Errorf("unexpected outbox: %+v", events)
	}
}

func TestStripeFailureIsPersistedWithFailureOutbox(t *testing.T) {
	db := testutil.NewDB(t, database.Migrate)
	usecase := NewPaymentUseCase(db, &fakeStripe{err: &paymentmethod.PermanentError{Err: errors.New("declined")}})

	result, err := usecase.Create(context.Background(), cardCommand())
	if err != nil || !errors.Is(result.Rejected, ErrProviderUnavailable) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Payment.Status != model.PaymentStatusFailed {
		t.Errorf("status = %s, want failed", result.Payment.Status)
	}
	var event model.OutboxEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Topic != orderflow.TopicPaymentFailed {
		t.Errorf("topic = %s, want %s", event.Topic, orderflow.TopicPaymentFailed)
	}
}
