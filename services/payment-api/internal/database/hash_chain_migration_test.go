package database_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/dbmigrate"
	"github.com/asamigentoku/PinguCoin/pkg/testutil"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/services/payment-api/migrations"
)

func TestHashChainMigrationBackfillsAndProtectsLegacyTransactions(t *testing.T) {
	db := testutil.NewDB(t, func(db *gorm.DB) error {
		all, err := dbmigrate.Load(migrations.FS, ".")
		if err != nil {
			return err
		}
		options := dbmigrate.Options{BaselineVersion: 1, SentinelTable: "payments", Table: "payment_schema_migrations"}
		if _, err := dbmigrate.Run(context.Background(), db, all[:3], options); err != nil {
			return err
		}
		if err := db.Exec("INSERT INTO point_accounts (user_id, balance) VALUES (7, 700)").Error; err != nil {
			return err
		}
		if err := db.Exec(`INSERT INTO point_transactions (user_id, amount, type, reason, balance_after) VALUES
			(7, 1000, 'credit', 'legacy top-up', 1000),
			(7, -300, 'payment', 'legacy purchase', 700)`).Error; err != nil {
			return err
		}
		_, err = dbmigrate.Run(context.Background(), db, all, options)
		return err
	})

	repo := repository.NewPointRepository(db)
	if err := repo.VerifyChain(7); err != nil {
		t.Fatalf("verify backfilled chain: %v", err)
	}
	var transactions []model.PointTransaction
	if err := db.Order("chain_sequence").Find(&transactions).Error; err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 2 || transactions[0].ChainSequence != 1 || transactions[1].PreviousHash != transactions[0].TransactionHash {
		t.Fatalf("unexpected backfilled chain: %+v", transactions)
	}
	if err := db.Exec("UPDATE point_transactions SET amount = amount + 1 WHERE id = ?", transactions[0].ID).Error; err == nil {
		t.Fatal("append-only trigger allowed a point transaction update")
	}
}
