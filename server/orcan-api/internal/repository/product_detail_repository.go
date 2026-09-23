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

func (repo *ProductDetailRepository) Create(detail *model.ProductDetail) error {
	return repo.db.Create(detail).Error
}

func (repo *ProductDetailRepository) FindAll(productID uint) ([]model.ProductDetail, error) {
	var details []model.ProductDetail
	query := repo.db.Order("sort_order, id")
	if productID != 0 {
		query = query.Where("product_id = ?", productID)
	}
	err := query.Find(&details).Error
	return details, err
}

func (repo *ProductDetailRepository) FindByID(id uint) (*model.ProductDetail, error) {
	var detail model.ProductDetail
	if err := repo.db.First(&detail, id).Error; err != nil {
		return nil, err
	}
	return &detail, nil
}

func (repo *ProductDetailRepository) Update(detail *model.ProductDetail) error {
	return repo.db.Save(detail).Error
}

func (repo *ProductDetailRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductDetail{}, id).Error
}

// DeleteByImageURL はproduct_id + image_urlに一致するレコードを削除する
// (アップロード済みのサブ画像ファイルを削除した際に、対応するレコードも消すために使う)。
func (repo *ProductDetailRepository) DeleteByImageURL(productID uint, imageURL string) error {
	return repo.db.Where("product_id = ? AND image_url = ?", productID, imageURL).Delete(&model.ProductDetail{}).Error
}
