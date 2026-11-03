package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrIdempotencyConflict = errors.New("idempotency key belongs to a different order")

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) CreateOrGet(candidate *model.Order, topic string) (*model.Order, bool, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(candidate).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(orderflow.OrderCreated{OrderID: candidate.ID, UserID: candidate.UserID, ProductID: candidate.ProductID, Quantity: candidate.Quantity, UnitPrice: candidate.UnitPrice, TotalAmount: candidate.TotalAmount, PaymentMethod: candidate.PaymentMethod, Currency: candidate.Currency, IdempotencyKey: candidate.IdempotencyKey})
		if err != nil {
			return err
		}
		return tx.Create(&model.OutboxEvent{AggregateID: candidate.ID, Topic: topic, EventKey: candidate.IdempotencyKey, Payload: payload}).Error
	})
	if err == nil {
		return candidate, false, nil
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil, false, err
	}
	var existing model.Order
	if e := r.db.Where("idempotency_key = ?", candidate.IdempotencyKey).First(&existing).Error; e != nil {
		return nil, false, e
	}
	if existing.UserID != candidate.UserID || existing.ProductID != candidate.ProductID || existing.Quantity != candidate.Quantity || existing.PaymentMethod != candidate.PaymentMethod || existing.Currency != candidate.Currency {
		return nil, false, ErrIdempotencyConflict
	}
	return &existing, true, nil
}

func (r *Repository) ApplyInventoryResult(ctx context.Context, orderID uint, reserved bool, reason string) error {
	status := "payment_pending"
	if !reserved {
		status = "failed"
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ? AND status IN ?", orderID, []string{"pending", "inventory_pending"}).Updates(map[string]any{"status": status, "failure_reason": reason}).Error
}

func (r *Repository) ApplyPaymentResult(ctx context.Context, orderID, paymentID uint, succeeded bool, reason string) error {
	status := "paid"
	if !succeeded {
		status = "failed"
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ? AND status IN ?", orderID, []string{"payment_pending", "payment_action_required", "pending"}).Updates(map[string]any{"status": status, "payment_id": paymentID, "failure_reason": reason, "action_url": ""}).Error
}

func (r *Repository) ApplyPaymentActionRequired(ctx context.Context, orderID, paymentID uint, actionURL string) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status IN ?", orderID, []string{"payment_pending", "pending", "payment_action_required"}).
		Updates(map[string]any{"status": "payment_action_required", "payment_id": paymentID, "action_url": actionURL}).Error
}
func (r *Repository) Find(id, user uint) (*model.Order, error) {
	var row model.Order
	e := r.db.Where("id = ? AND user_id = ?", id, user).First(&row).Error
	return &row, e
}
func (r *Repository) List(user uint) ([]model.Order, error) {
	var rows []model.Order
	e := r.db.Where("user_id = ?", user).Order("id desc").Find(&rows).Error
	return rows, e
}
func (r *Repository) HasPaid(user, product uint) (bool, error) {
	var count int64
	e := r.db.Model(&model.Order{}).Where("user_id = ? AND product_id = ? AND status = ?", user, product, "paid").Count(&count).Error
	return count > 0, e
}
func (r *Repository) Complete(order *model.Order, status string, paymentID uint, reason string, payload []byte, topic string) error {
	unitPrice, totalAmount := order.UnitPrice, order.TotalAmount
	return r.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(order, order.ID).Error; e != nil {
			return e
		}
		if order.Status != "processing" {
			return nil
		}
		if e := tx.Model(order).Updates(map[string]any{
			"status": status, "payment_id": paymentID, "failure_reason": reason,
			"unit_price": unitPrice, "total_amount": totalAmount,
		}).Error; e != nil {
			return e
		}
		order.Status, order.PaymentID, order.FailureReason = status, paymentID, reason
		order.UnitPrice, order.TotalAmount = unitPrice, totalAmount
		return tx.Create(&model.OutboxEvent{AggregateID: order.ID, Topic: topic, EventKey: order.IdempotencyKey, Payload: payload}).Error
	})
}
