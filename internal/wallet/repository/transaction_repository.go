package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/txmanager"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *entity.Transaction) error
	FindByID(ctx context.Context, id uint) (*entity.Transaction, error)
	FindByWalletID(ctx context.Context, walletID uint, filter dto.TransactionFilter) ([]entity.Transaction, int64, error)
}

type transactionRepositoryImpl struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepositoryImpl{db: db}
}

func (r *transactionRepositoryImpl) GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := txmanager.GetTxFromCtx(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *transactionRepositoryImpl) Create(ctx context.Context, transaction *entity.Transaction) error {
	err := r.GetTx(ctx).Create(transaction).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *transactionRepositoryImpl) FindByID(ctx context.Context, id uint) (*entity.Transaction, error) {
	var transaction entity.Transaction
	err := r.GetTx(ctx).First(&transaction, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepositoryImpl) FindByWalletID(ctx context.Context, walletID uint, filter dto.TransactionFilter) ([]entity.Transaction, int64, error) {
	var result []entity.Transaction
	var total int64
	query := r.GetTx(ctx).Model(&entity.Transaction{}).Where("wallet_id = ?", walletID)

	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if filter.StartDate != "" {
		query = query.Where("created_at >= ?", filter.StartDate+" 00:00:00")
	}
	if filter.EndDate != "" {
		query = query.Where("created_at <= ?", filter.EndDate+" 23:59:59")
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if filter.Limit == 0 {
		filter.Limit = 10
	}

	if filter.Page == 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.Limit
	err = query.Offset(offset).Limit(filter.Limit).Find(&result).Error
	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}
