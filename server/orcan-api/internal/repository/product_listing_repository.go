package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type ProductListingRepository struct {
	db *gorm.DB
}

func NewProductListingRepository(db *gorm.DB) *ProductListingRepository {
	return &ProductListingRepository{db: db}
}

func (r *ProductListingRepository) Create(l *model.ProductListing) error {
	return r.db.Create(l).Error
}

func (r *ProductListingRepository) FindAll(productID uint) ([]model.ProductListing, error) {
	var listings []model.ProductListing
	q := r.db.Order("id")
	if productID != 0 {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&listings).Error
	return listings, err
}

func (r *ProductListingRepository) FindByID(id uint) (*model.ProductListing, error) {
	var listing model.ProductListing
	if err := r.db.First(&listing, id).Error; err != nil {
		return nil, err
	}
	return &listing, nil
}

func (r *ProductListingRepository) Update(l *model.ProductListing) error {
	return r.db.Save(l).Error
}

func (r *ProductListingRepository) Delete(id uint) error {
	return r.db.Delete(&model.ProductListing{}, id).Error
}
