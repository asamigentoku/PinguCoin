package model

import "time"

// ProviderEvent makes webhook delivery idempotent. Providers retry webhooks,
// so the business transition and this receipt are committed together.
type ProviderEvent struct {
	ID        uint      `gorm:"primaryKey"`
	Provider  string    `gorm:"size:30;not null;uniqueIndex:idx_provider_events_provider_event"`
	EventID   string    `gorm:"size:255;not null;uniqueIndex:idx_provider_events_provider_event"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (ProviderEvent) TableName() string { return "provider_events" }
