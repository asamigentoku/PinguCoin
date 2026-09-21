package model

import "time"

// ProductDetail は商品の画像・詳細情報を表す(1商品に対して複数件)。
type ProductDetail struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ProductID   uint      `gorm:"not null;index" json:"product_id"`
	ImageURL    string    `gorm:"size:512" json:"image_url"`
	Description string    `gorm:"type:text" json:"description"`
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ProductDetail) TableName() string {
	return "product_detail"
}
