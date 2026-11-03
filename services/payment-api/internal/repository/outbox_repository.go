package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
)

type OutboxRepository struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) *OutboxRepository           { return &OutboxRepository{db: db} }
func (r *OutboxRepository) WithTx(tx *gorm.DB) *OutboxRepository  { return NewOutboxRepository(tx) }
func (r *OutboxRepository) Create(event *model.OutboxEvent) error { return r.db.Create(event).Error }

func (r *OutboxRepository) Pending(limit int) ([]model.OutboxEvent, error) {
	var events []model.OutboxEvent
	err := r.db.Where("published_at IS NULL").Order("id").Limit(limit).Find(&events).Error
	return events, err
}

func (r *OutboxRepository) MarkPublished(id uint) error {
	now := time.Now().UTC()
	return r.db.Model(&model.OutboxEvent{}).Where("id = ? AND published_at IS NULL", id).
		Updates(map[string]any{"published_at": now, "attempts": gorm.Expr("attempts + 1")}).Error
}

func (r *OutboxRepository) MarkAttempt(id uint) error {
	return r.db.Model(&model.OutboxEvent{}).Where("id = ?", id).Update("attempts", gorm.Expr("attempts + 1")).Error
}

func (r *OutboxRepository) LockPayment(tx *gorm.DB, id uint) error {
	var payment model.Payment
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, id).Error
}
