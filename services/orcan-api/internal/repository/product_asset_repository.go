package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
)

type ProductAssetRepository struct {
	db *gorm.DB
}

func NewProductAssetRepository(db *gorm.DB) *ProductAssetRepository {
	return &ProductAssetRepository{db: db}
}

func (repo *ProductAssetRepository) FindPurposeByID(id uint16) (*model.ProductAssetPurpose, error) {
	var purpose model.ProductAssetPurpose
	if err := repo.db.First(&purpose, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &purpose, nil
}

func (repo *ProductAssetRepository) FindAll(productID uint, purposeID uint16) ([]model.ProductAsset, error) {
	var assets []model.ProductAsset
	query := repo.db.Preload("Purpose").Where("product_id = ?", productID)
	if purposeID != 0 {
		query = query.Where("purpose_id = ?", purposeID)
	}
	err := query.Order("purpose_id, sort_order, id").Find(&assets).Error
	return assets, err
}

func (repo *ProductAssetRepository) FindByID(id uint) (*model.ProductAsset, error) {
	var asset model.ProductAsset
	if err := repo.db.Preload("Purpose").First(&asset, id).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (repo *ProductAssetRepository) Create(asset *model.ProductAsset) error {
	return repo.db.Transaction(func(tx *gorm.DB) error {
		if asset.IsPrimary {
			if err := tx.Model(&model.ProductAsset{}).
				Where("product_id = ? AND purpose_id = ?", asset.ProductID, asset.PurposeID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(asset).Error
	})
}

func (repo *ProductAssetRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductAsset{}, id).Error
}
