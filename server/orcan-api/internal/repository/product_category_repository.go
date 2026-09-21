package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type ProductCategoryRepository struct {
	db *gorm.DB
}

func NewProductCategoryRepository(db *gorm.DB) *ProductCategoryRepository {
	return &ProductCategoryRepository{db: db}
}

func (repo *ProductCategoryRepository) Create(category *model.ProductCategory) error {
	return repo.db.Create(category).Error
}

func (repo *ProductCategoryRepository) FindAll() ([]model.ProductCategory, error) {
	var categories []model.ProductCategory
	err := repo.db.Order("id").Find(&categories).Error
	return categories, err
}

func (repo *ProductCategoryRepository) FindByID(id uint) (*model.ProductCategory, error) {
	var category model.ProductCategory
	if err := repo.db.First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (repo *ProductCategoryRepository) Update(category *model.ProductCategory) error {
	return repo.db.Save(category).Error
}

func (repo *ProductCategoryRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductCategory{}, id).Error
}
