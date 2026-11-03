package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
)

type PaymentSourceRepository struct{ db *gorm.DB }

func NewPaymentSourceRepository(db *gorm.DB) *PaymentSourceRepository {
	return &PaymentSourceRepository{db: db}
}
func (r *PaymentSourceRepository) WithTx(tx *gorm.DB) *PaymentSourceRepository {
	return NewPaymentSourceRepository(tx)
}
func (r *PaymentSourceRepository) Create(source *model.PaymentSource) error {
	return r.db.Create(source).Error
}
func (r *PaymentSourceRepository) Update(source *model.PaymentSource) error {
	return r.db.Save(source).Error
}
func (r *PaymentSourceRepository) FindByPaymentID(paymentID uint) (*model.PaymentSource, error) {
	var source model.PaymentSource
	if err := r.db.Where("payment_id = ?", paymentID).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *PaymentSourceRepository) FindByProviderReference(reference string) (*model.PaymentSource, error) {
	var source model.PaymentSource
	if err := r.db.Where("provider_reference = ?", reference).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *PaymentSourceRepository) PendingBefore(before time.Time, limit int) ([]model.PaymentSource, error) {
	var sources []model.PaymentSource
	err := r.db.Where("status = ? AND created_at < ?", model.PaymentSourceStatusPending, before).Order("id").Limit(limit).Find(&sources).Error
	return sources, err
}
