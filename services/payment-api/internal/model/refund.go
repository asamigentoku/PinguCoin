package model

import "time"

// RefundStatus は返金のステータス。
const (
	RefundStatusPending   = "pending"
	RefundStatusSucceeded = "succeeded"
	RefundStatusFailed    = "failed"
)

// Refund は決済に対する返金(全額 or 一部)を表す。1決済に対して複数件になり得る。
type Refund struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PaymentID uint      `gorm:"not null;index" json:"payment_id"`
	Amount    int64     `gorm:"not null" json:"amount"`
	Reason    string    `gorm:"type:text" json:"reason"`
	Status    string    `gorm:"size:20;not null;default:'pending'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Refund) TableName() string {
	return "refunds"
}
