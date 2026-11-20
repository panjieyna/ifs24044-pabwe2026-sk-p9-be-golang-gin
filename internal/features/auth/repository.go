package auth

import (
	"errors"

	"gorm.io/gorm"

	"app/internal/database"
)

// UserRepository mengabstraksi akses data pengguna.
type UserRepository interface {
	FindByUsernameOrEmail(identifier string) (*database.User, error)
	FindByID(id uint) (*database.User, error)
	ExistsByUsername(username string) (bool, error)
	ExistsByEmail(email string, excludeID uint) (bool, error)
	Create(user *database.User) error
	Update(user *database.User) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository membuat UserRepository berbasis GORM.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByUsernameOrEmail(identifier string) (*database.User, error) {
	var user database.User
	if err := r.db.Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", identifier, identifier).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(id uint) (*database.User, error) {
	var user database.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).Where("LOWER(username) = LOWER(?)", username).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(email string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).Where("LOWER(email) = LOWER(?) AND id <> ?", email, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) Create(user *database.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) Update(user *database.User) error {
	return r.db.Save(user).Error
}

// isNotFound membantu membedakan "data tidak ada" dari error database lainnya.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
