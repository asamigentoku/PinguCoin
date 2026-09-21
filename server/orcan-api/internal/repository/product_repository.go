package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(p *model.Product) error {
	return r.db.Create(p).Error
}

// FindAll は商品一覧を返す。userID を指定すると出品者で絞り込む。
func (r *ProductRepository) FindAll(userID uint) ([]model.Product, error) {
	var products []model.Product
	q := r.db.Preload("Category").Order("id")
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	err := q.Find(&products).Error
	return products, err
}

func (r *ProductRepository) FindByID(id uint) (*model.Product, error) {
	var product model.Product
	if err := r.db.Preload("Category").First(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) Update(p *model.Product) error {
	return r.db.Save(p).Error
}

func (r *ProductRepository) Delete(id uint) error {
	return r.db.Delete(&model.Product{}, id).Error
}
