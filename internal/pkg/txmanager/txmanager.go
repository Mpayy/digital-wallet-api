package txmanager

import (
	"context"

	"gorm.io/gorm"
)

type TxManager interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type txManagerImpl struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) TxManager {
	return &txManagerImpl{db: db}
}

func (t *txManagerImpl) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return t.db.WithContext(ctx).Transaction(fn)
}
