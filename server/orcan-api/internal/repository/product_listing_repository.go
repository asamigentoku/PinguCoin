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

func (repo *ProductListingRepository) Create(listing *model.ProductListing) error {
	return repo.db.Create(listing).Error
}

func (repo *ProductListingRepository) FindAll(productID uint) ([]model.ProductListing, error) {
	var listings []model.ProductListing
	query := repo.db.Order("id")
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Find(&listings).Error
	return listings, err
}

func (repo *ProductListingRepository) FindByID(id uint) (*model.ProductListing, error) {
	var listing model.ProductListing
	if err := repo.db.First(&listing, id).Error; err != nil {
		return nil, err
	}
	return &listing, nil
}

func (repo *ProductListingRepository) Update(listing *model.ProductListing) error {
	return repo.db.Save(listing).Error
}

func (repo *ProductListingRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductListing{}, id).Error
}
