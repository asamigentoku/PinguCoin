package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type ProductDetailRepository struct {
	db *gorm.DB
}

func NewProductDetailRepository(db *gorm.DB) *ProductDetailRepository {
	return &ProductDetailRepository{db: db}
}

func (r *ProductDetailRepository) Create(d *model.ProductDetail) error {
	return r.db.Create(d).Error
}

func (r *ProductDetailRepository) FindAll(productID uint) ([]model.ProductDetail, error) {
	var details []model.ProductDetail
	q := r.db.Order("sort_order, id")
	if productID != 0 {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&details).Error
	return details, err
}

func (r *ProductDetailRepository) FindByID(id uint) (*model.ProductDetail, error) {
	var detail model.ProductDetail
	if err := r.db.First(&detail, id).Error; err != nil {
		return nil, err
	}
	return &detail, nil
}

func (r *ProductDetailRepository) Update(d *model.ProductDetail) error {
	return r.db.Save(d).Error
}

func (r *ProductDetailRepository) Delete(id uint) error {
	return r.db.Delete(&model.ProductDetail{}, id).Error
}
