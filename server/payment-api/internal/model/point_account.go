package model

import "time"

// PointAccount はユーザーごとのポイント残高(user_idごとに1件)。
type PointAccount struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	Balance   int64     `gorm:"not null;default:0" json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PointAccount) TableName() string {
	return "point_accounts"
}
