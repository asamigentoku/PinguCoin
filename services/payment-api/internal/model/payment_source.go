package model

import "time"

const (
	PaymentSourceStatusPending    = "pending"
	PaymentSourceStatusAuthorized = "authorized"
	PaymentSourceStatusCaptured   = "captured"
	PaymentSourceStatusFailed     = "failed"
	PaymentSourceStatusCanceled   = "canceled"
)

// PaymentSource records the provider-side half of an external payment.
// ProviderReference is the identifier used for idempotent reconciliation.
type PaymentSource struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	PaymentID         uint       `gorm:"not null;uniqueIndex" json:"payment_id"`
	Provider          string     `gorm:"size:30;not null" json:"provider"`
	ProviderReference string     `gorm:"size:255;not null;default:'';index" json:"provider_reference"`
	Status            string     `gorm:"size:20;not null;default:'pending'" json:"status"`
	FailureReason     string     `gorm:"type:text;not null;default:''" json:"failure_reason"`
	ActionURL         string     `gorm:"type:text;not null;default:''" json:"action_url"`
	OrderPayload      string     `gorm:"type:jsonb;not null;default:'{}'" json:"order_payload"`
	ExpiresAt         *time.Time `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (PaymentSource) TableName() string { return "payment_sources" }
