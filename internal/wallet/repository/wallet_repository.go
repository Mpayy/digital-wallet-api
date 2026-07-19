package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *entity.Wallet) error
	// insert wallet baru saldo 0, dipanggil saat auto-provision registrasi

	FindByUserID(ctx context.Context, userID uint) (*entity.Wallet, error)
	// resolve wallet TANPA lock — buat GET /wallets/me & resolve awal sender/recipient di Transfer

	LockByID(tx *gorm.DB, walletID uint) (*entity.Wallet, error)
	// SELECT ... FOR UPDATE by PK — WAJIB dipanggil di dalam DB transaction (topup & transfer)

	Save(tx *gorm.DB, wallet *entity.Wallet) error
	// persist balance yang sudah dimutasi di memory, dalam transaction yang sama dgn LockByID

	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
	// wrapper db.Transaction(fn) — bungkus 1 flow topup/transfer jadi 1 unit atomic
}

type walletRepositoryImpl struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepositoryImpl{db: db}
}

func (r *walletRepositoryImpl) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	err := r.db.WithContext(ctx).Transaction(fn)
	if err != nil {
		return err
	}

	return nil
}

func (r *walletRepositoryImpl) Create(ctx context.Context, wallet *entity.Wallet) error {
	err := r.db.WithContext(ctx).Create(wallet).Error
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
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}

		return nil, err
	}

	return &wallet, nil
}

func (r *walletRepositoryImpl) LockByID(tx *gorm.DB, walletID uint) (*entity.Wallet, error) {
	var wallet entity.Wallet
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&wallet, walletID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}

		return nil, err
	}

	return &wallet, nil
}

func (r *walletRepositoryImpl) Save(tx *gorm.DB, wallet *entity.Wallet) error {
	err := tx.Save(wallet).Error
	if err != nil {
		return err
	}

	return nil
}
