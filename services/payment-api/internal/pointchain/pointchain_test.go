package pointchain

import (
	"testing"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
)

func linkedTransactions() []model.PointTransaction {
	first := model.PointTransaction{
		UserID: 1, Amount: 1000, Type: model.PointTransactionTypeCredit,
		Reason: "top-up", BalanceAfter: 1000, ChainSequence: 1, PreviousHash: GenesisHash,
	}
	first.TransactionHash = Hash(&first)
	paymentID := uint(42)
	second := model.PointTransaction{
		UserID: 1, Amount: -300, Type: model.PointTransactionTypePayment, PaymentID: &paymentID,
		Reason: "purchase", BalanceAfter: 700, ChainSequence: 2, PreviousHash: first.TransactionHash,
	}
	second.TransactionHash = Hash(&second)
	return []model.PointTransaction{first, second}
}

func TestVerifyValidChain(t *testing.T) {
	if err := Verify(linkedTransactions()); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDetectsModifiedEntry(t *testing.T) {
	transactions := linkedTransactions()
	transactions[0].Amount++
	if err := Verify(transactions); err == nil {
		t.Fatal("modified amount was not detected")
	}
}

func TestVerifyDetectsBrokenBalanceAndLink(t *testing.T) {
	transactions := linkedTransactions()
	transactions[1].BalanceAfter++
	transactions[1].TransactionHash = Hash(&transactions[1])
	if err := Verify(transactions); err == nil {
		t.Fatal("broken balance continuity was not detected")
	}

	transactions = linkedTransactions()
	transactions[1].PreviousHash = GenesisHash
	transactions[1].TransactionHash = Hash(&transactions[1])
	if err := Verify(transactions); err == nil {
		t.Fatal("broken previous hash was not detected")
	}
}
