package model

import (
	"time"

	"gorm.io/gorm"
)

// ProductCategory は商品カテゴリを表す。
type ProductCategory struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ParentID  *uint          `gorm:"index" json:"parent_id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ProductCategory) TableName() string {
	return "product_categories"
}
