package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
)

// ErrInsufficientPoints はポイント残高が消費要求額に満たない場合に返る。
var ErrInsufficientPoints = errors.New("insufficient points")

type PointRepository struct {
	db *gorm.DB
}

func NewPointRepository(db *gorm.DB) *PointRepository {
	return &PointRepository{db: db}
}

// WithTx はトランザクション用の *gorm.DB に差し替えた同じRepositoryを返す。
// CreatePayment/RefundPayment のように「決済」と「ポイント増減」を1つの
// db.Transaction(...) にまとめて原子的に行いたい場合、その中で repo.WithTx(tx) して使う。
func (repo *PointRepository) WithTx(tx *gorm.DB) *PointRepository {
	return &PointRepository{db: tx}
}

// GetAccount はユーザーのポイント口座を取得する。まだ1件も取引がないユーザーは
// レコード自体が存在しない(gorm.ErrRecordNotFound)ので、呼び出し側で残高0として扱う。
func (repo *PointRepository) GetAccount(userID uint) (*model.PointAccount, error) {
	var account model.PointAccount
	if err := repo.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// ListTransactions はポイント増減履歴を返す。userIDを指定するとそのユーザーに絞り込む。
func (repo *PointRepository) ListTransactions(userID uint) ([]model.PointTransaction, error) {
	var transactions []model.PointTransaction
	query := repo.db.Order("id desc")
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Find(&transactions).Error
	return transactions, err
}

// Adjust はユーザーのポイント残高を amount だけ増減させ(正=付与, 負=消費)、
// 取引履歴(PointTransaction)を1件作成する。残高が不足する場合は ErrInsufficientPoints を返す。
//
// 行ロック(SELECT ... FOR UPDATE)を使うため、必ずトランザクション内(WithTxしたrepo)で
// 呼び出すこと。トランザクション外で呼ぶと同時消費に対するロックが効かず、
// 残高計算が競合する可能性がある。単体で使いたい場合は AdjustAtomic を使う。
func (repo *PointRepository) Adjust(userID uint, amount int64, txType, reason string, paymentID *uint) (*model.PointTransaction, error) {
	var account model.PointAccount
	err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&account).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		account = model.PointAccount{UserID: userID, Balance: 0}
		if err := repo.db.Create(&account).Error; err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}

	newBalance := account.Balance + amount
	if newBalance < 0 {
		return nil, ErrInsufficientPoints
	}

	if err := repo.db.Model(&account).Update("balance", newBalance).Error; err != nil {
		return nil, err
	}

	transaction := &model.PointTransaction{
		UserID:       userID,
		Amount:       amount,
		Type:         txType,
		PaymentID:    paymentID,
		Reason:       reason,
		BalanceAfter: newBalance,
	}
	if err := repo.db.Create(transaction).Error; err != nil {
		return nil, err
	}
	return transaction, nil
}

// AdjustAtomic はポイント増減だけで完結する操作(手動付与/消費)向けに、
// 自前でトランザクションを張ってから Adjust を呼ぶ。
func (repo *PointRepository) AdjustAtomic(userID uint, amount int64, txType, reason string, paymentID *uint) (*model.PointTransaction, error) {
	var result *model.PointTransaction
	err := repo.db.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = repo.WithTx(tx).Adjust(userID, amount, txType, reason, paymentID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
