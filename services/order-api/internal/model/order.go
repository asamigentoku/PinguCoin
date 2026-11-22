package model

import "time"

// Order は注文の現在状態と、購入対象・金額・決済情報を保持します。
type Order struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"not null;index"`
	ProductID      uint   `gorm:"not null;index"`
	Quantity       int64  `gorm:"not null"`
	UnitPrice      int64  `gorm:"not null"`
	TotalAmount    int64  `gorm:"not null"`
	PaymentID      uint   `gorm:"not null;default:0"`
	PaymentMethod  string `gorm:"size:30;not null;default:'point'"`
	Currency       string `gorm:"size:10;not null;default:'POINT'"`
	ActionURL      string `gorm:"type:text"`
	Status         string `gorm:"size:20;not null;default:'processing'"`
	IdempotencyKey string `gorm:"size:255;not null;uniqueIndex"`
	FailureReason  string `gorm:"size:255"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName は注文テーブル名を明示します。
func (Order) TableName() string { return "orders" }

// OutboxEvent はDB更新と同じトランザクションで保存するKafka発行待ちイベントです。
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

// TableName は注文イベントのoutboxテーブル名を明示します。
func (OutboxEvent) TableName() string { return "order_outbox" }
