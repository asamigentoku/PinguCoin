package repository

import (
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(u *model.User) error {
	return r.db.Create(u).Error
}

func (r *UserRepository) FindAll() ([]model.User, error) {
	var users []model.User
	err := r.db.Order("id").Find(&users).Error
	return users, err
}

func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByClerkUserID(clerkUserID string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("clerk_user_id = ?", clerkUserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(u *model.User) error {
	return r.db.Save(u).Error
}

func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&model.User{}, id).Error
}
