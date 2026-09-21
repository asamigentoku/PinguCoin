package model

import (
	"time"

	"gorm.io/gorm"
)

// User はサービス利用者のアプリ内プロフィールを表す。認証はClerk(フロントエンド)が担うため、
// orcan-apiはパスワード等の認証情報を一切保持せず、ClerkUserIDでClerkのユーザーと1:1に対応する。
type User struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ClerkUserID string         `gorm:"size:255;not null;uniqueIndex" json:"clerk_user_id"`
	Email       string         `gorm:"size:255;not null" json:"email"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}
