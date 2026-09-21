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

func (repo *UserRepository) Create(user *model.User) error {
	return repo.db.Create(user).Error
}

func (repo *UserRepository) FindAll() ([]model.User, error) {
	var users []model.User
	err := repo.db.Order("id").Find(&users).Error
	return users, err
}

func (repo *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := repo.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (repo *UserRepository) FindByClerkUserID(clerkUserID string) (*model.User, error) {
	var user model.User
	if err := repo.db.Where("clerk_user_id = ?", clerkUserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (repo *UserRepository) Update(user *model.User) error {
	return repo.db.Save(user).Error
}

func (repo *UserRepository) Delete(id uint) error {
	return repo.db.Delete(&model.User{}, id).Error
}
