package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/pointchain"
)

// ErrInsufficientPoints はポイント残高が消費要求額に満たない場合に返る。
// ErrInsufficientPointsは、減算後の残高が0未満になる場合に返します。
var ErrInsufficientPoints = errors.New("insufficient points")

// ErrPointChainIntegrityは、ポイント台帳のハッシュチェーンまたは残高に不整合がある場合に返します。
var ErrPointChainIntegrity = errors.New("point transaction hash chain integrity violation")

// PointRepositoryはポイント残高とポイント取引履歴をDBで管理します。
// point_accountsは現在残高、point_transactionsは残高変更の履歴を保持します。
type PointRepository struct {
	db *gorm.DB
}

func NewPointRepository(db *gorm.DB) *PointRepository {
	return &PointRepository{db: db}
}

// WithTx はトランザクション用の *gorm.DB に差し替えた同じRepositoryを返す。
// CreatePayment/RefundPayment のように「決済」と「ポイント増減」を1つの
// db.Transaction(...) にまとめて原子的に行いたい場合、その中で repo.WithTx(tx) して使う。
// WithTxは既存のトランザクションを使うPointRepositoryを作成します。
// 決済レコード、ポイント残高、ポイント履歴を同時に確定したい場合に使用します。
func (repo *PointRepository) WithTx(tx *gorm.DB) *PointRepository {
	return &PointRepository{db: tx}
}

// GetAccount はユーザーのポイント口座を取得する。まだ1件も取引がないユーザーは
// レコード自体が存在しない(gorm.ErrRecordNotFound)ので、呼び出し側で残高0として扱う。
// GetAccountはpoint_accountsからユーザーの現在残高を取得します。
func (repo *PointRepository) GetAccount(userID uint) (*model.PointAccount, error) {
	var account model.PointAccount
	if err := repo.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// ListTransactions はポイント増減履歴を返す。userIDを指定するとそのユーザーに絞り込む。
// ListTransactionsはポイント履歴を取得する前に、台帳の整合性を検証します。
func (repo *PointRepository) ListTransactions(userID uint) ([]model.PointTransaction, error) {
	if err := repo.VerifyChain(userID); err != nil {
		return nil, err
	}
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
// Adjustはポイント残高とポイント履歴を同時に更新します。
// amountが正なら付与、負なら消費です。行ロックを使うため、呼び出し元のトランザクション内で実行します。
func (repo *PointRepository) Adjust(userID uint, amount int64, txType, reason string, paymentID *uint) (*model.PointTransaction, error) {
	// point_accountsの行をロックし、同じユーザーへの同時更新を直列化します。
	var account model.PointAccount
	//行ロック特定のユーザーのレコードのみブロック
	err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&account).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 初回利用ユーザーには残高0のアカウントを作成します。
		account = model.PointAccount{UserID: userID, Balance: 0}
		if err := repo.db.Create(&account).Error; err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	if err := repo.VerifyChain(userID); err != nil {
		// 既存台帳が壊れている場合は、残高を変更せず処理を中止します。
		return nil, err
	}

	newBalance := account.Balance + amount
	if newBalance < 0 {
		// 残高不足の場合は、accountもtransactionも変更しません。
		return nil, ErrInsufficientPoints
	}

	// 高速参照用の現在残高を更新します。
	if err := repo.db.Model(&account).Update("balance", newBalance).Error; err != nil {
		return nil, err
	}

	transaction := &model.PointTransaction{
		// 変更量だけでなく、変更後残高と理由も履歴に残します。
		UserID:       userID,
		Amount:       amount,
		Type:         txType,
		PaymentID:    paymentID,
		Reason:       reason,
		BalanceAfter: newBalance,
	}
	var previous model.PointTransaction
	err = repo.db.Where("user_id = ?", userID).Order("chain_sequence DESC").First(&previous).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 最初の取引はGenesisHashからチェーンを開始します。
		transaction.ChainSequence = 1
		transaction.PreviousHash = pointchain.GenesisHash
	case err != nil:
		return nil, err
	default:
		// 直前の取引ハッシュを参照して、履歴を一本のチェーンにします。
		transaction.ChainSequence = previous.ChainSequence + 1
		transaction.PreviousHash = previous.TransactionHash
	}
	// 取引内容からハッシュを計算し、後から改ざんを検出できるようにします。
	transaction.TransactionHash = pointchain.Hash(transaction)
	// 残高変更の履歴をpoint_transactionsへ保存します。
	if err := repo.db.Create(transaction).Error; err != nil {
		return nil, err
	}
	return transaction, nil
}

// VerifyChain recalculates the selected users' complete point ledgers.
// A zero userID verifies every user's chain.
// VerifyChainはDBからポイント台帳を読み込み、次の2段階で整合性を検証します。
// 1. pointchain.Verifyで、順序・前ハッシュ・残高計算・各レコードのハッシュを確認する。
// 2. point_accountsの現在残高と、台帳上の最新BalanceAfterを確認する。
// userIDが0の場合は全ユーザー、0以外の場合は指定ユーザーだけを検証します。
func (repo *PointRepository) VerifyChain(userID uint) error {
	// pointchain.Verifyが検証できるよう、ユーザー・チェーン番号の古い順で取得します。
	var transactions []model.PointTransaction
	query := repo.db.Order("user_id ASC, chain_sequence ASC")
	if userID != 0 {
		// 指定ユーザーだけを検証する場合は、対象ユーザーの履歴に絞ります。
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&transactions).Error; err != nil {
		// 台帳を読み取れない場合は、検証できないためエラーにします。
		return err
	}
	if err := pointchain.Verify(transactions); err != nil {
		// 履歴の順序、ハッシュリンク、残高計算のいずれかが壊れています。
		// 1件でも改ざん・欠落・順序不整合があれば、残高操作を止めます。
		return fmt.Errorf("%w: %v", ErrPointChainIntegrity, err)
	}
	latestBalances := make(map[uint]int64)
	for i := range transactions {
		// 各ユーザーについて、台帳上で最後に記録された残高を取り出します。
		latestBalances[transactions[i].UserID] = transactions[i].BalanceAfter
	}
	var accounts []model.PointAccount
	accountsQuery := repo.db.Order("user_id ASC")
	if userID != 0 {
		accountsQuery = accountsQuery.Where("user_id = ?", userID)
	}
	if err := accountsQuery.Find(&accounts).Error; err != nil {
		// 比較対象となるpoint_accountsを読み取れない場合も検証失敗です。
		return err
	}
	accountBalances := make(map[uint]int64, len(accounts))
	for i := range accounts {
		// point_accountsは高速参照用のキャッシュなので、台帳の最新残高と一致する必要があります。
		accountBalances[accounts[i].UserID] = accounts[i].Balance
		if accounts[i].Balance != latestBalances[accounts[i].UserID] {
			return fmt.Errorf("%w: user %d account balance does not match the ledger", ErrPointChainIntegrity, accounts[i].UserID)
		}
	}
	for ledgerUser := range latestBalances {
		if _, exists := accountBalances[ledgerUser]; !exists {
			// 履歴が存在するのに残高アカウントがない状態は不整合です。
			return fmt.Errorf("%w: user %d account is missing", ErrPointChainIntegrity, ledgerUser)
		}
	}
	return nil
}

// AdjustAtomic はポイント増減だけで完結する操作(手動付与/消費)向けに、
// 自前でトランザクションを張ってから Adjust を呼ぶ。
// AdjustAtomicは呼び出し元がトランザクションを用意しない場合の簡易APIです。
// 残高更新と履歴保存を1つのトランザクションで実行します。
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
