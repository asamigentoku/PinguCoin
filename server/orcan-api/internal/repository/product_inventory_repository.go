package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

// ErrInsufficientStock は在庫数(quantity)が消費要求量に満たない場合に返る。
var ErrInsufficientStock = errors.New("insufficient stock")

type ProductInventoryRepository struct {
	db *gorm.DB
}

func NewProductInventoryRepository(db *gorm.DB) *ProductInventoryRepository {
	return &ProductInventoryRepository{db: db}
}

// WithTx はトランザクション用の *gorm.DB に差し替えた同じRepositoryを返す。
// Adjust のように行ロックを取りつつ在庫と履歴を1トランザクションで更新したい場合に使う。
func (repo *ProductInventoryRepository) WithTx(tx *gorm.DB) *ProductInventoryRepository {
	return &ProductInventoryRepository{db: tx}
}

func (repo *ProductInventoryRepository) Create(inventory *model.ProductInventory) error {
	return repo.db.Create(inventory).Error
}

func (repo *ProductInventoryRepository) FindAll() ([]model.ProductInventory, error) {
	var inventories []model.ProductInventory
	err := repo.db.Order("id").Find(&inventories).Error
	return inventories, err
}

func (repo *ProductInventoryRepository) FindByID(id uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := repo.db.First(&inventory, id).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (repo *ProductInventoryRepository) FindByProductID(productID uint) (*model.ProductInventory, error) {
	var inventory model.ProductInventory
	if err := repo.db.Where("product_id = ?", productID).First(&inventory).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

func (repo *ProductInventoryRepository) Update(inventory *model.ProductInventory) error {
	return repo.db.Save(inventory).Error
}

func (repo *ProductInventoryRepository) Delete(id uint) error {
	return repo.db.Delete(&model.ProductInventory{}, id).Error
}

// Adjust は商品の在庫数(quantity)を amount だけ増減させ(負=消費、正=戻し)、
// 増減履歴(ProductInventoryTransaction)を1件作成する。在庫が不足する場合は
// ErrInsufficientStock を返す。
//
// idempotencyKey に一致する履歴が既にあれば、新たな増減は行わずそのレコードを
// そのまま返す(2番目の戻り値がtrue)。呼び出し元がリトライしても二重に
// 在庫が減る/戻ることはない。
//
// 行ロック(SELECT ... FOR UPDATE)を使うため、必ずトランザクション内(WithTxしたrepo)で
// 呼び出すこと。単体で使いたい場合は AdjustAtomic を使う。
func (repo *ProductInventoryRepository) Adjust(productID uint, amount int, reason, idempotencyKey string) (transaction *model.ProductInventoryTransaction, replayed bool, err error) {
	var existing model.ProductInventoryTransaction
	err = repo.db.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
	switch {
	case err == nil:
		return &existing, true, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, err
	}

	var inventory model.ProductInventory
	if err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ?", productID).
		First(&inventory).Error; err != nil {
		return nil, false, err
	}

	newQuantity := inventory.Quantity + amount
	if newQuantity < 0 {
		return nil, false, ErrInsufficientStock
	}

	if err := repo.db.Model(&inventory).Update("quantity", newQuantity).Error; err != nil {
		return nil, false, err
	}

	newTransaction := &model.ProductInventoryTransaction{
		ProductID:      productID,
		Amount:         amount,
		Reason:         reason,
		IdempotencyKey: idempotencyKey,
		QuantityAfter:  newQuantity,
	}
	// 同時に同じidempotencyKeyでリクエストが来た場合の競合に備え、
	// ユニーク制約違反(gorm.ErrDuplicatedKey)は「先に成功した方の結果を返す」形で救済する。
	if err := repo.db.Create(newTransaction).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			var raced model.ProductInventoryTransaction
			if findErr := repo.db.Where("idempotency_key = ?", idempotencyKey).First(&raced).Error; findErr != nil {
				return nil, false, findErr
			}
			return &raced, true, nil
		}
		return nil, false, err
	}
	return newTransaction, false, nil
}

// AdjustAtomic は在庫増減だけで完結する操作向けに、自前でトランザクションを張ってから
// Adjust を呼ぶ。
func (repo *ProductInventoryRepository) AdjustAtomic(productID uint, amount int, reason, idempotencyKey string) (transaction *model.ProductInventoryTransaction, replayed bool, err error) {
	txErr := repo.db.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		transaction, replayed, innerErr = repo.WithTx(tx).Adjust(productID, amount, reason, idempotencyKey)
		return innerErr
	})
	if txErr != nil {
		return nil, false, txErr
	}
	return transaction, replayed, nil
}
