package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(tx *gorm.DB, transaction *entity.Transaction) error
	// insert baris transaksi — dipanggil dalam DB transaction yang sama dgn WalletRepository.Save

	FindByID(ctx context.Context, id uint) (*entity.Transaction, error)
	// untuk GET /transactions/:id

	FindByWalletID(ctx context.Context, walletID uint, filter dto.TransactionFilter) ([]entity.Transaction, int64, error)
	// list riwayat dgn pagination + filter type/date — untuk GET /transactions, sekalian total count
}

type transactionRepositoryImpl struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepositoryImpl{db: db}
}

func (r *transactionRepositoryImpl) Create(tx *gorm.DB, transaction *entity.Transaction) error {
	err := tx.Create(transaction).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *transactionRepositoryImpl) FindByID(ctx context.Context, id uint) (*entity.Transaction, error) {
	var transaction entity.Transaction
	err := r.db.WithContext(ctx).First(&transaction, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepositoryImpl) FindByWalletID(ctx context.Context, walletID uint, filter dto.TransactionFilter) ([]entity.Transaction, int64, error) {
	applyFilter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("wallet_id = ?", walletID)
		if filter.Type != "" {
			q = q.Where("type = ?", filter.Type)
		}

		if filter.StartDate != "" {
			q = q.Where("created_at >= ?", filter.StartDate+" 00:00:00")
		}

		if filter.EndDate != "" {
			q = q.Where("created_at <= ?", filter.EndDate+" 23:59:59")
		}

		return q
	}

	var total int64
	if err := applyFilter(r.db.WithContext(ctx).Model(&entity.Transaction{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit == 0 {
		filter.Limit = 10
	}

	if filter.Page == 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.Limit

	var result []entity.Transaction
	q := applyFilter(r.db.WithContext(ctx).Model(&entity.Transaction{}))
	if err := q.Order("created_at DESC").Offset(offset).Limit(filter.Limit).Find(&result).Error; err != nil {
		return nil, 0, err
	}

	return result, total, nil
}
