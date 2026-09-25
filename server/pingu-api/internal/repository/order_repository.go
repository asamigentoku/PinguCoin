package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/model"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (repo *OrderRepository) Create(order *model.Order) error {
	return repo.db.Create(order).Error
}

func (repo *OrderRepository) FindByID(id uint) (*model.Order, error) {
	var order model.Order
	if err := repo.db.First(&order, id).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByIdempotencyKey はidempotency_keyに一致する注文を返す(無ければgorm.ErrRecordNotFound)。
func (repo *OrderRepository) FindByIdempotencyKey(idempotencyKey string) (*model.Order, error) {
	var order model.Order
	if err := repo.db.Where("idempotency_key = ?", idempotencyKey).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByUser はuserIDが購入した注文の一覧を新しい順に返す。
func (repo *OrderRepository) FindByUser(userID uint) ([]model.Order, error) {
	var orders []model.Order
	err := repo.db.Where("user_id = ?", userID).Order("id desc").Find(&orders).Error
	return orders, err
}

// HasPaidOrder はuserIDがproductIDを支払い済み(status="paid")で注文したことがあるかを返す。
// 商品ファイルのダウンロード許可判定に使う。
func (repo *OrderRepository) HasPaidOrder(userID, productID uint) (bool, error) {
	var count int64
	err := repo.db.Model(&model.Order{}).
		Where("user_id = ? AND product_id = ? AND status = ?", userID, productID, "paid").
		Count(&count).Error
	return count > 0, err
}
