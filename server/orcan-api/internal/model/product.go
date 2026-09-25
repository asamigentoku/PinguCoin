package model

import (
	"time"

	"gorm.io/gorm"
)

// Product は商品を表す。
type Product struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	UserID      uint            `gorm:"not null;index" json:"user_id"`
	CategoryID  uint            `gorm:"not null;index" json:"category_id"`
	Category    ProductCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Name        string          `gorm:"size:255;not null" json:"name"`
	Description string          `gorm:"type:text" json:"description"`
	ImageURL    string          `gorm:"size:512" json:"image_url"`
	FileURL     string          `gorm:"size:512" json:"file_url"`
	Price       int64           `gorm:"not null" json:"price"`
	Status      string          `gorm:"size:20;not null;default:'draft'" json:"status"`
	// Version は更新のたびに1ずつ増える値。キャッシュのキー/無効化判定(このレコードが
	// 変わったかどうか)に使う。更新処理自体は楽観ロックせず、BeforeUpdateフックで
	// 無条件にインクリメントするだけ(同時更新の競合検出目的ではない)。
	Version   uint           `gorm:"not null;default:1" json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Product) TableName() string {
	return "products"
}

// BeforeUpdate はレコード更新のたびにVersionを1つ進める。
func (product *Product) BeforeUpdate(tx *gorm.DB) error {
	tx.Statement.SetColumn("version", product.Version+1)
	return nil
}
