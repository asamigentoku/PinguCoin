package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/paymentmethod"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
)

var (
	ErrInvalid             = errors.New("invalid payment")
	ErrInsufficientPoints  = errors.New("insufficient points")
	ErrUnsupportedMethod   = errors.New("unsupported payment method")
	ErrProviderUnavailable = errors.New("payment provider unavailable")
)

type CreatePaymentCommand struct {
	UserID, ProductID uint
	Amount            int64
	Currency          string
	PaymentMethod     string
	IdempotencyKey    string
	Order             *orderflow.OrderCreated // present for Kafka-triggered payments
	Purpose           string
}

type CreatePaymentResult struct {
	Payment   *model.Payment
	Replayed  bool
	Rejected  error
	ActionURL string
}

type PaymentUseCase struct {
	db        *gorm.DB
	payments  *repository.PaymentRepository
	points    *repository.PointRepository
	sources   *repository.PaymentSourceRepository
	outbox    *repository.OutboxRepository
	stripe    paymentmethod.Charger
	returnURL string
}

func NewPaymentUseCase(db *gorm.DB, stripe paymentmethod.Charger, returnURLs ...string) *PaymentUseCase {
	returnURL := "http://localhost:3000/purchases"
	if len(returnURLs) > 0 && strings.TrimSpace(returnURLs[0]) != "" {
		returnURL = returnURLs[0]
	}
	return &PaymentUseCase{db: db, payments: repository.NewPaymentRepository(db), points: repository.NewPointRepository(db), sources: repository.NewPaymentSourceRepository(db), outbox: repository.NewOutboxRepository(db), stripe: stripe, returnURL: returnURL}
}

func (u *PaymentUseCase) Create(ctx context.Context, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	cmd.Currency, cmd.PaymentMethod, cmd.IdempotencyKey = strings.TrimSpace(cmd.Currency), strings.TrimSpace(cmd.PaymentMethod), strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Purpose == "" {
		cmd.Purpose = model.PaymentPurposePurchase
	}
	if err := validate(cmd); err != nil {
		return &CreatePaymentResult{Rejected: err}, nil
	}
	db := u.db.WithContext(ctx)
	if existing, err := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); err == nil {
		return u.replay(db, existing), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	switch cmd.PaymentMethod {
	case model.PaymentMethodPoint:
		return u.createPoint(db, cmd)
	case model.PaymentMethodCard, model.PaymentMethodCreditCard, model.PaymentMethodBankTransfer:
		return u.createStripe(ctx, db, cmd)
	default:
		result := &CreatePaymentResult{Rejected: fmt.Errorf("%w: %s", ErrUnsupportedMethod, cmd.PaymentMethod)}
		if cmd.Order != nil {
			result.Payment, _ = u.recordRejected(db, cmd, result.Rejected)
		}
		return result, nil
	}
}

func validate(c CreatePaymentCommand) error {
	switch {
	case c.UserID == 0:
		return fmt.Errorf("%w: user_id is required", ErrInvalid)
	case c.ProductID == 0 && c.Purpose != model.PaymentPurposePointTopUp:
		return fmt.Errorf("%w: product_id is required", ErrInvalid)
	case c.Amount <= 0:
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalid)
	case c.Currency == "":
		return fmt.Errorf("%w: currency is required", ErrInvalid)
	case c.PaymentMethod == "":
		return fmt.Errorf("%w: payment_method is required", ErrInvalid)
	case c.IdempotencyKey == "":
		return fmt.Errorf("%w: idempotency_key is required", ErrInvalid)
	case c.Purpose != model.PaymentPurposePurchase && c.Purpose != model.PaymentPurposePointTopUp:
		return fmt.Errorf("%w: invalid purpose", ErrInvalid)
	case c.PaymentMethod == model.PaymentMethodPoint && (c.Currency != "POINT" || c.Purpose != model.PaymentPurposePurchase):
		return fmt.Errorf("%w: point payments require currency POINT and purpose purchase", ErrInvalid)
	case (c.PaymentMethod == model.PaymentMethodCard || c.PaymentMethod == model.PaymentMethodCreditCard || c.PaymentMethod == model.PaymentMethodBankTransfer) && c.Currency != "JPY":
		return fmt.Errorf("%w: external payments require currency JPY", ErrInvalid)
	default:
		return nil
	}
}

func newPayment(c CreatePaymentCommand, status string) *model.Payment {
	return &model.Payment{UserID: c.UserID, ProductID: c.ProductID, Amount: c.Amount, Currency: c.Currency, PaymentMethod: c.PaymentMethod, Purpose: c.Purpose, Status: status, IdempotencyKey: c.IdempotencyKey}
}

func replay(payment *model.Payment) *CreatePaymentResult {
	r := &CreatePaymentResult{Payment: payment, Replayed: true}
	if payment.Status == model.PaymentStatusFailed {
		r.Rejected = errors.New("payment previously failed")
	}
	return r
}

func (u *PaymentUseCase) replay(db *gorm.DB, payment *model.Payment) *CreatePaymentResult {
	r := replay(payment)
	if source, err := u.sources.WithTx(db).FindByPaymentID(payment.ID); err == nil {
		r.ActionURL = source.ActionURL
	}
	return r
}

func (u *PaymentUseCase) createPoint(db *gorm.DB, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	payment := newPayment(cmd, model.PaymentStatusSucceeded)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		paymentID := payment.ID
		if _, err := u.points.WithTx(tx).Adjust(payment.UserID, -payment.Amount, model.PointTransactionTypePayment, fmt.Sprintf("payment #%d", payment.ID), &paymentID); err != nil {
			return err
		}
		return u.enqueue(tx, payment, cmd, "")
	})
	if errors.Is(err, repository.ErrInsufficientPoints) {
		rejected := ErrInsufficientPoints
		if cmd.Order != nil {
			payment, _ = u.recordRejected(db, cmd, rejected)
		} else {
			payment = nil
		}
		return &CreatePaymentResult{Payment: payment, Rejected: rejected}, nil
	}
	if err != nil {
		if existing, findErr := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); findErr == nil {
			return u.replay(db, existing), nil
		}
		return nil, err
	}
	return &CreatePaymentResult{Payment: payment}, nil
}

func (u *PaymentUseCase) createStripe(ctx context.Context, db *gorm.DB, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	payment := newPayment(cmd, model.PaymentStatusProcessing)
	source := &model.PaymentSource{Provider: "stripe", Status: model.PaymentSourceStatusPending}
	if cmd.Order != nil {
		payload, err := json.Marshal(cmd.Order)
		if err != nil {
			return nil, err
		}
		source.OrderPayload = string(payload)
	} else {
		source.OrderPayload = "{}"
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		source.PaymentID = payment.ID
		return u.sources.WithTx(tx).Create(source)
	}); err != nil {
		if existing, findErr := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); findErr == nil {
			return u.replay(db, existing), nil
		}
		return nil, err
	}

	returnURL := u.returnURL
	if cmd.Purpose == model.PaymentPurposePointTopUp {
		returnURL = strings.TrimSuffix(returnURL, "/purchases") + "/points"
	}
	charged, chargeErr := u.stripe.Charge(ctx, paymentmethod.ChargeRequest{Amount: cmd.Amount, Currency: cmd.Currency, IdempotencyKey: cmd.IdempotencyKey, ReturnURL: returnURL, PaymentMethod: cmd.PaymentMethod})
	if chargeErr != nil {
		if !paymentmethod.IsPermanent(chargeErr) {
			// The provider may have completed the charge even though the response was
			// lost. Keep processing and let the idempotent reconciler converge it.
			return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, chargeErr)
		}
		rejected := fmt.Errorf("%w: %v", ErrProviderUnavailable, chargeErr)
		if err := db.Transaction(func(tx *gorm.DB) error {
			payment.Status, source.Status, source.FailureReason = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, rejected.Error()
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueue(tx, payment, cmd, rejected.Error())
		}); err != nil {
			return nil, err
		}
		return &CreatePaymentResult{Payment: payment, Rejected: rejected}, nil
	}

	if charged.Status == "requires_action" {
		if err := db.Transaction(func(tx *gorm.DB) error {
			source.Status, source.ProviderReference, source.ActionURL = model.PaymentSourceStatusAuthorized, charged.ID, charged.ActionURL
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueueActionRequired(tx, payment, cmd, charged.ActionURL)
		}); err != nil {
			return nil, err
		}
		return &CreatePaymentResult{Payment: payment, ActionURL: charged.ActionURL}, nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		payment.Status = model.PaymentStatusSucceeded
		source.Status, source.ProviderReference = model.PaymentSourceStatusCaptured, charged.ID
		if err := u.payments.WithTx(tx).Update(payment); err != nil {
			return err
		}
		if err := u.sources.WithTx(tx).Update(source); err != nil {
			return err
		}
		if payment.Purpose == model.PaymentPurposePointTopUp {
			paymentID := payment.ID
			if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
				return err
			}
		}
		return u.enqueue(tx, payment, cmd, "")
	}); err != nil {
		_ = u.stripe.Cancel(ctx, charged.ID) // compensation; reconciliation retries unresolved cases
		return nil, err
	}
	return &CreatePaymentResult{Payment: payment}, nil
}

func (u *PaymentUseCase) enqueueActionRequired(tx *gorm.DB, payment *model.Payment, cmd CreatePaymentCommand, actionURL string) error {
	if cmd.Order == nil {
		return nil
	}
	event := orderflow.PaymentResult{OrderCreated: *cmd.Order, PaymentID: payment.ID, ActionURL: actionURL}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return u.outbox.WithTx(tx).Create(&model.OutboxEvent{AggregateID: payment.ID, Topic: orderflow.TopicPaymentActionRequired, Key: cmd.IdempotencyKey, Payload: string(payload)})
}

func (u *PaymentUseCase) recordRejected(db *gorm.DB, cmd CreatePaymentCommand, reason error) (*model.Payment, error) {
	payment := newPayment(cmd, model.PaymentStatusFailed)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		return u.enqueue(tx, payment, cmd, reason.Error())
	})
	return payment, err
}

func (u *PaymentUseCase) enqueue(tx *gorm.DB, payment *model.Payment, cmd CreatePaymentCommand, reason string) error {
	if cmd.Order == nil {
		return nil
	}
	event := orderflow.PaymentResult{OrderCreated: *cmd.Order, PaymentID: payment.ID, Reason: reason}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	topic := orderflow.TopicPaymentSucceeded
	if reason != "" || payment.Status == model.PaymentStatusFailed {
		topic = orderflow.TopicPaymentFailed
	}
	return u.outbox.WithTx(tx).Create(&model.OutboxEvent{AggregateID: payment.ID, Topic: topic, Key: cmd.IdempotencyKey, Payload: string(payload)})
}

// ReconcileProcessing closes the crash window between provider capture and the
// local final transaction. Stripe is called with the original idempotency key.
func (u *PaymentUseCase) ReconcileProcessing(ctx context.Context, olderThan time.Duration) error {
	db := u.db.WithContext(ctx)
	sources, err := u.sources.WithTx(db).PendingBefore(time.Now().Add(-olderThan), 100)
	if err != nil {
		return err
	}
	for i := range sources {
		source := &sources[i]
		payment, err := u.payments.WithTx(db).FindByID(source.PaymentID)
		if err != nil {
			return err
		}
		if payment.Status != model.PaymentStatusProcessing {
			continue
		}
		var order *orderflow.OrderCreated
		if source.OrderPayload != "" && source.OrderPayload != "{}" {
			var decoded orderflow.OrderCreated
			if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
				order = &decoded
			}
		}
		cmd := CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Order: order}
		charged, err := u.stripe.Charge(ctx, paymentmethod.ChargeRequest{Amount: payment.Amount, Currency: payment.Currency, IdempotencyKey: payment.IdempotencyKey})
		if err != nil {
			if !paymentmethod.IsPermanent(err) {
				continue
			}
			reason := fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
			if err := db.Transaction(func(tx *gorm.DB) error {
				payment.Status, source.Status, source.FailureReason = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, reason.Error()
				if err := u.payments.WithTx(tx).Update(payment); err != nil {
					return err
				}
				if err := u.sources.WithTx(tx).Update(source); err != nil {
					return err
				}
				return u.enqueue(tx, payment, cmd, reason.Error())
			}); err != nil {
				return err
			}
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			payment.Status = model.PaymentStatusSucceeded
			source.Status, source.ProviderReference = model.PaymentSourceStatusCaptured, charged.ID
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			if payment.Purpose == model.PaymentPurposePointTopUp {
				paymentID := payment.ID
				if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
					return err
				}
			}
			return u.enqueue(tx, payment, cmd, "")
		}); err != nil {
			_ = u.stripe.Cancel(ctx, charged.ID)
			return err
		}
	}
	return nil
}

type ProviderEvent struct {
	ID   string                     `json:"id"`
	Type string                     `json:"type"`
	Data paymentmethod.ChargeResult `json:"data"`
}

// HandleProviderEvent applies a retried webhook exactly once. The receipt,
// payment transition, point top-up and outbox event share one transaction.
func (u *PaymentUseCase) HandleProviderEvent(ctx context.Context, event ProviderEvent) error {
	if event.ID == "" || event.Data.ID == "" {
		return fmt.Errorf("%w: invalid provider event", ErrInvalid)
	}
	db := u.db.WithContext(ctx)
	source, err := u.sources.WithTx(db).FindByProviderReference(event.Data.ID)
	if err != nil {
		return err
	}
	payment, err := u.payments.WithTx(db).FindByID(source.PaymentID)
	if err != nil {
		return err
	}
	var order *orderflow.OrderCreated
	if source.OrderPayload != "" && source.OrderPayload != "{}" {
		var decoded orderflow.OrderCreated
		if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
			order = &decoded
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		receipt := &model.ProviderEvent{Provider: source.Provider, EventID: event.ID}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(receipt)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return nil
		}
		if payment.Status != model.PaymentStatusProcessing {
			return nil
		}
		cmd := CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Purpose: payment.Purpose, Order: order}
		switch event.Type {
		case "payment_intent.succeeded":
			payment.Status, source.Status, source.ActionURL = model.PaymentStatusSucceeded, model.PaymentSourceStatusCaptured, ""
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			if payment.Purpose == model.PaymentPurposePointTopUp {
				paymentID := payment.ID
				if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
					return err
				}
			}
			return u.enqueue(tx, payment, cmd, "")
		case "payment_intent.failed", "payment_intent.canceled":
			payment.Status, source.Status, source.ActionURL = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, ""
			source.FailureReason = event.Type
			if event.Type == "payment_intent.canceled" {
				payment.Status, source.Status = model.PaymentStatusCanceled, model.PaymentSourceStatusCanceled
			}
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueue(tx, payment, cmd, event.Type)
		default:
			return fmt.Errorf("%w: unsupported provider event %s", ErrInvalid, event.Type)
		}
	})
}

func (u *PaymentUseCase) CancelExternal(ctx context.Context, payment *model.Payment) error {
	source, err := u.sources.WithTx(u.db.WithContext(ctx)).FindByPaymentID(payment.ID)
	if err != nil {
		return err
	}
	if source.ProviderReference != "" {
		if err := u.stripe.Cancel(ctx, source.ProviderReference); err != nil {
			return err
		}
	}
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		payment.Status, source.Status, source.ActionURL = model.PaymentStatusCanceled, model.PaymentSourceStatusCanceled, ""
		if err := u.payments.WithTx(tx).Update(payment); err != nil {
			return err
		}
		if err := u.sources.WithTx(tx).Update(source); err != nil {
			return err
		}
		var order *orderflow.OrderCreated
		if source.OrderPayload != "" && source.OrderPayload != "{}" {
			var decoded orderflow.OrderCreated
			if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
				order = &decoded
			}
		}
		return u.enqueue(tx, payment, CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Purpose: payment.Purpose, Order: order}, "payment canceled")
	})
}

func (u *PaymentUseCase) RefundExternal(ctx context.Context, payment *model.Payment, amount int64, key string) error {
	source, err := u.sources.WithTx(u.db.WithContext(ctx)).FindByPaymentID(payment.ID)
	if err != nil {
		return err
	}
	_, err = u.stripe.Refund(ctx, paymentmethod.RefundRequest{PaymentIntentID: source.ProviderReference, Amount: amount, IdempotencyKey: key})
	return err
}
