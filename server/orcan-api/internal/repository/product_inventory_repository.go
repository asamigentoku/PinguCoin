package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type ProductInventoryRepository struct {
	db *gorm.DB
}

func NewProductInventoryRepository(db *gorm.DB) *ProductInventoryRepository {
	return &ProductInventoryRepository{db: db}
}

func (r *ProductInventoryRepository) Create(i *model.ProductInventory) error {
	return r.db.Create(i).Error
}

func (r *ProductInventoryRepository) FindAll() ([]model.ProductInventory, error) {
	var inventories []model.ProductInventory
	err := r.db.Order("id").Find(&inventories).Error
	return inventories, err
}

func (r *ProductInventoryRepository) FindByID(id uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := r.db.First(&inventory, id).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *ProductInventoryRepository) FindByProductID(productID uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := r.db.Where("product_id = ?", productID).First(&inventory).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (r *ProductInventoryRepository) Update(i *model.ProductInventory) error {
	return r.db.Save(i).Error
}

func (r *ProductInventoryRepository) Delete(id uint) error {
	return r.db.Delete(&model.ProductInventory{}, id).Error
}
