package txmanager

import (
	"context"

	"gorm.io/gorm"
)

type contextKey string

const txKey contextKey = "tx"

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManagerImpl struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) TxManager {
	return &txManagerImpl{db: db}
}

func (t *txManagerImpl) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey, tx)
		return fn(txCtx)
	})
}

func GetTxFromCtx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txKey).(*gorm.DB)
	return tx, ok
}
