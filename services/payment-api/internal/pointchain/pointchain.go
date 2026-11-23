package pointchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
)

const (
	domain      = "PinguCoin.PointTransaction.v1"
	GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

func writeInt64(buffer *bytes.Buffer, value int64) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func writeUint64(buffer *bytes.Buffer, value uint64) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func writeString(buffer *bytes.Buffer, value string) {
	_ = binary.Write(buffer, binary.BigEndian, uint32(len([]byte(value))))
	buffer.WriteString(value)
}

// Hash returns the versioned, deterministic SHA-256 digest for one ledger entry.
func Hash(transaction *model.PointTransaction) string {
	var canonical bytes.Buffer
	canonical.WriteString(domain)
	writeUint64(&canonical, transaction.ChainSequence)
	writeUint64(&canonical, uint64(transaction.UserID))
	writeInt64(&canonical, transaction.Amount)
	writeString(&canonical, transaction.Type)
	if transaction.PaymentID == nil {
		canonical.WriteByte(0)
	} else {
		canonical.WriteByte(1)
		writeUint64(&canonical, uint64(*transaction.PaymentID))
	}
	writeString(&canonical, transaction.Reason)
	writeInt64(&canonical, transaction.BalanceAfter)
	previous, err := hex.DecodeString(transaction.PreviousHash)
	if err != nil || len(previous) != sha256.Size {
		return ""
	}
	canonical.Write(previous)
	digest := sha256.Sum256(canonical.Bytes())
	return hex.EncodeToString(digest[:])
}

// Verifyは、ポイント台帳の各レコードが正しい順序で並び、
// 前のレコードとハッシュでつながり、残高の計算が連続し、
// レコード自身のハッシュも改ざんされていないことを検証します。
//
// transactionsはuser_id、chain_sequenceの昇順で渡されることを前提にします。
// ユーザーが切り替わると、そのユーザーの台帳をGenesisHashから検証し直します。
func Verify(transactions []model.PointTransaction) error {
	var currentUser uint
	var hasCurrentUser bool
	var expectedSequence uint64
	var previousHash string
	var previousBalance int64
	for i := range transactions {
		transaction := &transactions[i]
		if !hasCurrentUser || transaction.UserID != currentUser {
			// ユーザーごとに独立した台帳なので、ユーザーが変わったら検証状態を初期化します。
			currentUser = transaction.UserID
			hasCurrentUser = true
			expectedSequence = 1
			previousHash = GenesisHash
			previousBalance = 0
		}
		if transaction.ChainSequence != expectedSequence {
			// 取引番号に欠番・重複・順序逆転がないか確認します。
			return fmt.Errorf("user %d: sequence is %d, want %d", transaction.UserID, transaction.ChainSequence, expectedSequence)
		}
		if transaction.PreviousHash != previousHash {
			// 現在の取引が、直前の取引またはGenesisHashを参照しているか確認します。
			return fmt.Errorf("user %d sequence %d: previous hash mismatch", transaction.UserID, transaction.ChainSequence)
		}
		if transaction.BalanceAfter != previousBalance+transaction.Amount {
			// 「前回残高 + 今回の増減額」と、保存された残高が一致するか確認します。
			return fmt.Errorf("user %d sequence %d: balance continuity mismatch", transaction.UserID, transaction.ChainSequence)
		}
		if calculated := Hash(transaction); calculated == "" || transaction.TransactionHash != calculated {
			// 保存済みの内容からハッシュを再計算し、内容が後から変更されていないか確認します。
			return fmt.Errorf("user %d sequence %d: transaction hash mismatch", transaction.UserID, transaction.ChainSequence)
		}
		// 次のレコードを検証するため、今回の番号・ハッシュ・残高を基準値にします。
		expectedSequence++
		previousHash = transaction.TransactionHash
		previousBalance = transaction.BalanceAfter
	}
	return nil
}
