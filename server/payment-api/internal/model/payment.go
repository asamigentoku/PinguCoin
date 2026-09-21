package model

import (
	"time"

	"gorm.io/gorm"
)

// PaymentStatus は決済のステータス。
const (
	PaymentStatusPending           = "pending"
	PaymentStatusSucceeded         = "succeeded"
	PaymentStatusFailed            = "failed"
	PaymentStatusCanceled          = "canceled"
	PaymentStatusRefunded          = "refunded"
	PaymentStatusPartiallyRefunded = "partially_refunded"
)

// Payment は1回の決済を表す。
type Payment struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;index" json:"user_id"`
	// orcan-api の products.id への参照。サービスをまたぐため外部キーは張らずID参照のみ。
	ProductID     uint           `gorm:"not null;index" json:"product_id"`
	Amount        int64          `gorm:"not null" json:"amount"`
	Currency      string         `gorm:"size:10;not null" json:"currency"`
	PaymentMethod string         `gorm:"size:30;not null" json:"payment_method"`
	Status        string         `gorm:"size:20;not null;default:'pending'" json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Payment) TableName() string {
	return "payments"
}
