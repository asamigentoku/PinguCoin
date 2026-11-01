package model

import "time"

type Order struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"not null;index"`
	ProductID      uint   `gorm:"not null;index"`
	Quantity       int64  `gorm:"not null"`
	UnitPrice      int64  `gorm:"not null"`
	TotalAmount    int64  `gorm:"not null"`
	PaymentID      uint   `gorm:"not null;default:0"`
	Status         string `gorm:"size:20;not null;default:'processing'"`
	IdempotencyKey string `gorm:"size:255;not null;uniqueIndex"`
	FailureReason  string `gorm:"size:255"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Order) TableName() string { return "orders" }

type OutboxEvent struct {
	ID          uint   `gorm:"primaryKey"`
	AggregateID uint   `gorm:"not null;index"`
	Topic       string `gorm:"size:255;not null"`
	EventKey    string `gorm:"size:255;not null"`
	Payload     []byte `gorm:"type:jsonb;not null"`
	PublishedAt *time.Time
	Attempts    int    `gorm:"not null;default:0"`
	LastError   string `gorm:"type:text"`
	CreatedAt   time.Time
}

func (OutboxEvent) TableName() string { return "order_outbox" }
