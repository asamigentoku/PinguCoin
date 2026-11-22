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

// ErrIdempotencyConflict は同じidempotency_keyを異なる注文内容で再利用した場合のエラーです。
var ErrIdempotencyConflict = errors.New("idempotency key belongs to a different order")

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

// CreateOrGet は注文と注文作成イベントを同一トランザクションで保存します。
// 同じidempotency_keyが存在する場合、同一内容なら既存注文を返します。
func (r *Repository) CreateOrGet(candidate *model.Order, topic string) (*model.Order, bool, error) {
    //ここでorderとorder_outboxを2つ同時にトランザクション
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

// ApplyInventoryResult は在庫確保の結果を注文ステータスへ反映します。
func (r *Repository) ApplyInventoryResult(ctx context.Context, orderID uint, reserved bool, reason string) error {
	status := "payment_pending"
	if !reserved {
		status = "failed"
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ? AND status IN ?", orderID, []string{"pending", "inventory_pending"}).Updates(map[string]any{"status": status, "failure_reason": reason}).Error
}

// ApplyPaymentResult は決済成功または失敗の結果を注文へ反映します。
func (r *Repository) ApplyPaymentResult(ctx context.Context, orderID, paymentID uint, succeeded bool, reason string) error {
	status := "paid"
	if !succeeded {
		status = "failed"
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ? AND status IN ?", orderID, []string{"payment_pending", "payment_action_required", "pending"}).Updates(map[string]any{"status": status, "payment_id": paymentID, "failure_reason": reason, "action_url": ""}).Error
}

// ApplyPaymentActionRequired は追加認証など、ユーザー操作が必要な状態を保存します。
func (r *Repository) ApplyPaymentActionRequired(ctx context.Context, orderID, paymentID uint, actionURL string) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status IN ?", orderID, []string{"payment_pending", "pending", "payment_action_required"}).
		Updates(map[string]any{"status": "payment_action_required", "payment_id": paymentID, "action_url": actionURL}).Error
}

// Find はユーザー所有の注文だけを取得します。
func (r *Repository) Find(id, user uint) (*model.Order, error) {
	var row model.Order
	e := r.db.Where("id = ? AND user_id = ?", id, user).First(&row).Error
	return &row, e
}

// List はユーザーの注文履歴を新しい順で取得します。
func (r *Repository) List(user uint) ([]model.Order, error) {
	var rows []model.Order
	e := r.db.Where("user_id = ?", user).Order("id desc").Find(&rows).Error
	return rows, e
}

// HasPaid は指定ユーザー・商品にpaid状態の注文が存在するかを確認します。
func (r *Repository) HasPaid(user, product uint) (bool, error) {
	var count int64
	e := r.db.Model(&model.Order{}).Where("user_id = ? AND product_id = ? AND status = ?", user, product, "paid").Count(&count).Error
	return count > 0, e
}

// Complete は注文状態の更新と後続イベントのoutbox保存を同一トランザクションで行います。
// 行ロックと状態確認により、同じ注文を複数回完了処理しないようにします。
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
