package repository

import (
	"context"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/txmanager"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
)

type TransferRepository interface {
	Create(ctx context.Context, transfer *entity.Transfer) error
}

type transferRepositoryImpl struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) TransferRepository {
	return &transferRepositoryImpl{db: db}
}
	
func (r *transferRepositoryImpl) GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := txmanager.GetTxFromCtx(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *transferRepositoryImpl) Create(ctx context.Context, transfer *entity.Transfer) error {
	err := r.GetTx(ctx).Create(transfer).Error
	if err != nil {
		return err
	}
	return nil
}