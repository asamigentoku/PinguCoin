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

func (repo *ProductRepository) Create(product *model.Product) error {
	return repo.db.Create(product).Error
}

// FindAll は商品一覧を返す。userID を指定すると出品者で絞り込む。
func (repo *ProductRepository) FindAll(userID uint) ([]model.Product, error) {
	var products []model.Product
	query := repo.db.Preload("Category").Order("id")
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Find(&products).Error
	return products, err
}

func (repo *ProductRepository) FindByID(id uint) (*model.Product, error) {
	var product model.Product
	if err := repo.db.Preload("Category").First(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (repo *ProductRepository) Update(product *model.Product) error {
	return repo.db.Save(product).Error
}

func (repo *ProductRepository) Delete(id uint) error {
	return repo.db.Delete(&model.Product{}, id).Error
}
