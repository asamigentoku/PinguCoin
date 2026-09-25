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

// WithTx はトランザクション用の *gorm.DB に差し替えた同じRepositoryを返す。
// point払いの決済のように、ポイント増減と1つのトランザクションにまとめたい場合に使う。
func (repo *PaymentRepository) WithTx(tx *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: tx}
}

func (repo *PaymentRepository) Create(payment *model.Payment) error {
	return repo.db.Create(payment).Error
}

// FindAll は決済履歴を返す。userIDを指定するとそのユーザーの決済に絞り込む。
func (repo *PaymentRepository) FindAll(userID uint) ([]model.Payment, error) {
	var payments []model.Payment
	query := repo.db.Order("id desc")
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Find(&payments).Error
	return payments, err
}

func (repo *PaymentRepository) FindByID(id uint) (*model.Payment, error) {
	var payment model.Payment
	if err := repo.db.First(&payment, id).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

// FindByIdempotencyKey はidempotency_keyに一致する決済を返す(無ければgorm.ErrRecordNotFound)。
func (repo *PaymentRepository) FindByIdempotencyKey(idempotencyKey string) (*model.Payment, error) {
	var payment model.Payment
	if err := repo.db.Where("idempotency_key = ?", idempotencyKey).First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (repo *PaymentRepository) Update(payment *model.Payment) error {
	return repo.db.Save(payment).Error
}
