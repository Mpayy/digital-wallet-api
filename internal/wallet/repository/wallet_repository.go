package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/txmanager"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *entity.Wallet) error
	FindByUserID(ctx context.Context, userID uint) (*entity.Wallet, error)
	LockByID(ctx context.Context, walletID uint) (*entity.Wallet, error)
	Save(ctx context.Context, wallet *entity.Wallet) error
}

type walletRepositoryImpl struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepositoryImpl{db: db}
}

func (r *walletRepositoryImpl) GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := txmanager.GetTxFromCtx(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *walletRepositoryImpl) Create(ctx context.Context, wallet *entity.Wallet) error {
	err := r.GetTx(ctx).Create(wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperror.ErrDuplicatedKey
		}
		return err
	}
	return nil
}

func (r *walletRepositoryImpl) FindByUserID(ctx context.Context, userID uint) (*entity.Wallet, error) {
	var wallet entity.Wallet
	err := r.GetTx(ctx).Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepositoryImpl) LockByID(ctx context.Context, walletID uint) (*entity.Wallet, error) {
	var wallet entity.Wallet
	err := r.GetTx(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&wallet, walletID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepositoryImpl) Save(ctx context.Context, wallet *entity.Wallet) error {
	err := r.GetTx(ctx).Save(wallet).Error
	if err != nil {
		return err
	}
	return nil
}
