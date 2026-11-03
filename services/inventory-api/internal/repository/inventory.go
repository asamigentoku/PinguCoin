package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/model"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type InventoryRepository struct{ db *gorm.DB }

func New(db *gorm.DB) *InventoryRepository { return &InventoryRepository{db: db} }

func (r *InventoryRepository) FindAll() ([]model.Inventory, error) {
	var rows []model.Inventory
	err := r.db.Order("id").Find(&rows).Error
	return rows, err
}

func (r *InventoryRepository) FindByID(id uint) (*model.Inventory, error) {
	var row model.Inventory
	if err := r.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *InventoryRepository) Create(row *model.Inventory) error { return r.db.Create(row).Error }
func (r *InventoryRepository) Update(row *model.Inventory) error { return r.db.Save(row).Error }
func (r *InventoryRepository) Delete(id uint) error              { return r.db.Delete(&model.Inventory{}, id).Error }

func (r *InventoryRepository) Adjust(productID uint, amount int, reason, key string) (row *model.Inventory, replayed bool, err error) {
	return r.AdjustContext(context.Background(), productID, amount, reason, key)
}

func (r *InventoryRepository) AdjustContext(ctx context.Context, productID uint, amount int, reason, key string) (row *model.Inventory, replayed bool, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event model.Transaction
		findErr := tx.Where("idempotency_key = ?", key).First(&event).Error
		if findErr == nil {
			replayed = true
			return tx.Where("product_id = ?", productID).First(&row).Error
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		var inventory model.Inventory
		findErr = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", productID).First(&inventory).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			inventory = model.Inventory{ProductID: productID, Quantity: model.DefaultDigitalStock}
			if createErr := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&inventory).Error; createErr != nil {
				return createErr
			}
			if inventory.ID == 0 {
				if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", productID).First(&inventory).Error; lockErr != nil {
					return lockErr
				}
			}
		} else if findErr != nil {
			return findErr
		}

		next := inventory.Quantity + amount
		if next < 0 {
			return ErrInsufficientStock
		}
		if updateErr := tx.Model(&inventory).Updates(map[string]any{"quantity": next, "version": gorm.Expr("version + 1")}).Error; updateErr != nil {
			return updateErr
		}
		if createErr := tx.Create(&model.Transaction{ProductID: productID, Amount: amount, Reason: reason, IdempotencyKey: key, QuantityAfter: next}).Error; createErr != nil {
			return createErr
		}
		inventory.Quantity = next
		inventory.Version++
		row = &inventory
		return nil
	})
	return row, replayed, err
}
