package tx

import (
	"context"

	"gorm.io/gorm"
)

type contextKey string

const txKey contextKey = "tx"

type Tx interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txImpl struct {
	db *gorm.DB
}

func NewTx(db *gorm.DB) Tx {
	return &txImpl{db: db}
}

func (t *txImpl) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey, tx)
		return fn(txCtx)
	})
}

func GetTxFromCtx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txKey).(*gorm.DB)
	return tx, ok
}
