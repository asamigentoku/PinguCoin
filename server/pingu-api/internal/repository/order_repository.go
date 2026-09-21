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

func (r *OrderRepository) Create(o *model.Order) error {
	return r.db.Create(o).Error
}

func (r *OrderRepository) FindByID(id uint) (*model.Order, error) {
	var order model.Order
	if err := r.db.First(&order, id).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByUser はuserIDが購入した注文の一覧を新しい順に返す。
func (r *OrderRepository) FindByUser(userID uint) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.Where("user_id = ?", userID).Order("id desc").Find(&orders).Error
	return orders, err
}
