package model

import "time"

// ProductListing は商品の出品情報を表す(1商品に対して複数件の出品が可能)。
type ProductListing struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	ProductID uint       `gorm:"not null;index" json:"product_id"`
	Price     int64      `gorm:"not null" json:"price"`
	Status    string     `gorm:"size:20;not null;default:'active'" json:"status"`
	ListedAt  time.Time  `json:"listed_at"`
	EndedAt   *time.Time `json:"ended_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (ProductListing) TableName() string {
	return "product_listings"
}
