package model

import "time"

// OutboxEvent is committed in the same transaction as the payment state.
// Delivery is at-least-once; consumers must deduplicate by Key.
type OutboxEvent struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	AggregateID uint       `gorm:"not null;index" json:"aggregate_id"`
	Topic       string     `gorm:"size:100;not null" json:"topic"`
	Key         string     `gorm:"size:255;not null" json:"key"`
	Payload     string     `gorm:"type:jsonb;not null" json:"payload"`
	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	PublishedAt *time.Time `gorm:"index" json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (OutboxEvent) TableName() string { return "outbox_events" }
