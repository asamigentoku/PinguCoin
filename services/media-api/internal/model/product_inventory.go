package model

import (
	"time"

	"gorm.io/gorm"
)

// ProductInventory は商品の在庫を表す(1商品につき1件)。
type ProductInventory struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ProductID uint `gorm:"not null;uniqueIndex" json:"product_id"`
	Quantity  int  `gorm:"not null;default:0" json:"quantity"`
	Reserved  int  `gorm:"not null;default:0" json:"reserved"`
	// Version は更新のたびに1ずつ増える値。キャッシュのキー/無効化判定に使う
	// (products.versionと同じ考え方。詳細はそちらのコメントを参照)。
	Version   uint      `gorm:"not null;default:1" json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProductInventory) TableName() string {
	return "product_inventory"
}

// BeforeUpdate はレコード更新のたびにVersionを1つ進める。
func (inventory *ProductInventory) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("version", inventory.Version+1)
	return nil
}
