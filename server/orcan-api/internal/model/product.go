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
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (Product) TableName() string {
	return "products"
}
