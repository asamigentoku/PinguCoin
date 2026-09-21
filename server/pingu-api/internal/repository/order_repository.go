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

// FindByUser はuserIDが購入した注文の一覧を新しい順に返す。
func (repo *OrderRepository) FindByUser(userID uint) ([]model.Order, error) {
	var orders []model.Order
	err := repo.db.Where("user_id = ?", userID).Order("id desc").Find(&orders).Error
	return orders, err
}
