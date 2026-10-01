package model

import "time"

// ProductAssetPurpose defines an extensible, database-backed asset purpose.
// New purposes can be introduced by inserting a row instead of changing the schema.
type ProductAssetPurpose struct {
	ID        uint16    `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Name      string    `gorm:"size:64;not null;uniqueIndex" json:"name"`
	IsPublic  bool      `gorm:"not null;default:false" json:"is_public"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProductAssetPurpose) TableName() string {
	return "product_asset_purposes"
}

const (
	ProductAssetPurposeProductImage uint16 = 1
	ProductAssetPurposeDetailImage  uint16 = 2
	ProductAssetPurposeProductFile  uint16 = 3
)

// ProductAsset is one uploaded object belonging to a product. Every purpose is
// one-to-many; PurposeID describes how the object is used.
type ProductAsset struct {
	ID               uint                `gorm:"primaryKey" json:"id"`
	ProductID        uint                `gorm:"not null;index:idx_product_assets_product_purpose_sort,priority:1" json:"product_id"`
	Product          Product             `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	PurposeID        uint16              `gorm:"not null;index:idx_product_assets_product_purpose_sort,priority:2" json:"purpose_id"`
	Purpose          ProductAssetPurpose `gorm:"foreignKey:PurposeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"purpose,omitempty"`
	StorageURL       string              `gorm:"size:1024;not null;uniqueIndex" json:"storage_url"`
	OriginalFilename string              `gorm:"size:255" json:"original_filename"`
	ContentType      string              `gorm:"size:255" json:"content_type"`
	FileSize         int64               `gorm:"not null;default:0" json:"file_size"`
	Description      string              `gorm:"type:text" json:"description"`
	SortOrder        int                 `gorm:"not null;default:0;index:idx_product_assets_product_purpose_sort,priority:3" json:"sort_order"`
	IsPrimary        bool                `gorm:"not null;default:false" json:"is_primary"`
	Metadata         string              `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func (ProductAsset) TableName() string {
	return "product_assets"
}
