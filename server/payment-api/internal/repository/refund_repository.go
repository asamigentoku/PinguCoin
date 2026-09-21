package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
)

type RefundRepository struct {
	db *gorm.DB
}

func NewRefundRepository(db *gorm.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

// WithTx はトランザクション用の *gorm.DB に差し替えた同じRepositoryを返す。
func (r *RefundRepository) WithTx(tx *gorm.DB) *RefundRepository {
	return &RefundRepository{db: tx}
}

func (r *RefundRepository) Create(refund *model.Refund) error {
	return r.db.Create(refund).Error
}

// FindAll は返金履歴を返す。paymentIDを指定するとその決済の返金に絞り込む。
func (r *RefundRepository) FindAll(paymentID uint) ([]model.Refund, error) {
	var refunds []model.Refund
	q := r.db.Order("id desc")
	if paymentID != 0 {
		q = q.Where("payment_id = ?", paymentID)
	}
	err := q.Find(&refunds).Error
	return refunds, err
}

// SumSucceededAmount は指定した決済に対して既に成功した返金額の合計を返す。
// RefundPayment で「返金しすぎ」を防ぐバリデーションに使う。
func (r *RefundRepository) SumSucceededAmount(paymentID uint) (int64, error) {
	var total int64
	err := r.db.Model(&model.Refund{}).
		Where("payment_id = ? AND status = ?", paymentID, model.RefundStatusSucceeded).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}
