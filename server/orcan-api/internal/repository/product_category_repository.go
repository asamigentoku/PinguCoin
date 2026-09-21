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

func (r *ProductCategoryRepository) Create(c *model.ProductCategory) error {
	return r.db.Create(c).Error
}

func (r *ProductCategoryRepository) FindAll() ([]model.ProductCategory, error) {
	var categories []model.ProductCategory
	err := r.db.Order("id").Find(&categories).Error
	return categories, err
}

func (r *ProductCategoryRepository) FindByID(id uint) (*model.ProductCategory, error) {
	var category model.ProductCategory
	if err := r.db.First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *ProductCategoryRepository) Update(c *model.ProductCategory) error {
	return r.db.Save(c).Error
}

func (r *ProductCategoryRepository) Delete(id uint) error {
	return r.db.Delete(&model.ProductCategory{}, id).Error
}
