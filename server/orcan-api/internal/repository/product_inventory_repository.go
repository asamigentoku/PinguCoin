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

func (repo *ProductInventoryRepository) Create(inventory *model.ProductInventory) error {
	return repo.db.Create(inventory).Error
}

func (repo *ProductInventoryRepository) FindAll() ([]model.ProductInventory, error) {
	var inventories []model.ProductInventory
	err := repo.db.Order("id").Find(&inventories).Error
	return inventories, err
}

func (repo *ProductInventoryRepository) FindByID(id uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := repo.db.First(&inventory, id).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (repo *ProductInventoryRepository) FindByProductID(productID uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := repo.db.Where("product_id = ?", productID).First(&inventory).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (repo *ProductInventoryRepository) Update(inventory *model.ProductInventory) error {
	return repo.db.Save(inventory).Error
}

func (repo *ProductInventoryRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductInventory{}, id).Error
}
