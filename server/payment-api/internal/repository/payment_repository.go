package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
)

type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(p *model.Payment) error {
	return r.db.Create(p).Error
}

// FindAll は決済履歴を返す。userIDを指定するとそのユーザーの決済に絞り込む。
func (r *PaymentRepository) FindAll(userID uint) ([]model.Payment, error) {
	var payments []model.Payment
	q := r.db.Order("id desc")
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	err := q.Find(&payments).Error
	return payments, err
}

func (r *PaymentRepository) FindByID(id uint) (*model.Payment, error) {
	var payment model.Payment
	if err := r.db.First(&payment, id).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *PaymentRepository) Update(p *model.Payment) error {
	return r.db.Save(p).Error
}
