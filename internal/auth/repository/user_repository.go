package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/auth/entity"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByID(ctx context.Context, id uint) (*entity.User, error)
}

type UserRepositoryImpl struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &UserRepositoryImpl{DB: db}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *entity.User) error {
	err := r.DB.WithContext(ctx).Create(user).Error
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return apperror.ErrDuplicatedKey
		}
		return err
	}
	return nil
}

func (r *UserRepositoryImpl) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) FindByID(ctx context.Context, id uint) (*entity.User, error) {
	var user entity.User
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}
