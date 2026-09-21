package model

import "time"

// ProductInventory は商品の在庫を表す(1商品につき1件)。
type ProductInventory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"not null;uniqueIndex" json:"product_id"`
	Quantity  int       `gorm:"not null;default:0" json:"quantity"`
	Reserved  int       `gorm:"not null;default:0" json:"reserved"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProductInventory) TableName() string {
	return "product_inventory"
}
