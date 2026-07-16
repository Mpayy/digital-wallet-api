package repository

import (
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
)

type TransferRepository interface {
	Create(tx *gorm.DB, transfer *entity.Transfer) error
}

type transferRepositoryImpl struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) TransferRepository {
	return &transferRepositoryImpl{db: db}
}

func (r *transferRepositoryImpl) Create(tx *gorm.DB, transfer *entity.Transfer) error {
	err := tx.Create(transfer).Error
	if err != nil {
		return err
	}
	return nil
}
