package model

import "time"

// PointTransactionType はポイント増減の種別。
const (
	PointTransactionTypeCredit  = "credit"  // 手動付与(キャンペーン等)
	PointTransactionTypeDebit   = "debit"   // 手動消費
	PointTransactionTypePayment = "payment" // point払いの決済による消費
	PointTransactionTypeRefund  = "refund"  // point払い決済の返金による付与
)

// PointTransaction はポイントの増減履歴。1件が1回の付与/消費に対応する。
type PointTransaction struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"not null;index" json:"user_id"`
	Amount int64  `gorm:"not null" json:"amount"` // 正の値=付与、負の値=消費
	Type   string `gorm:"size:20;not null" json:"type"`
	// Paymentに紐づく増減(point払い/その返金)の場合のみ設定される。
	PaymentID    *uint     `gorm:"index" json:"payment_id"`
	Reason       string    `gorm:"type:text" json:"reason"`
	BalanceAfter int64     `gorm:"not null" json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

func (PointTransaction) TableName() string {
	return "point_transactions"
}
