package model

import "time"

const DefaultDigitalStock = 1_000_000

type Inventory struct {
	ID        uint `gorm:"primaryKey"`
	ProductID uint `gorm:"not null;uniqueIndex"`
	Quantity  int  `gorm:"not null"`
	Reserved  int  `gorm:"not null;default:0"`
	Version   uint `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Inventory) TableName() string { return "product_inventory" }

type Transaction struct {
	ID             uint   `gorm:"primaryKey"`
	ProductID      uint   `gorm:"not null;index"`
	Amount         int    `gorm:"not null"`
	Reason         string `gorm:"size:255"`
	IdempotencyKey string `gorm:"size:255;not null;uniqueIndex"`
	QuantityAfter  int    `gorm:"not null"`
	CreatedAt      time.Time
}

func (Transaction) TableName() string { return "product_inventory_transactions" }
