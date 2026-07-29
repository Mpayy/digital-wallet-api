package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"gorm.io/gorm"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *entity.PaymentTransaction) error
	FindByProviderRefID(ctx context.Context, providerRefID string) (*entity.PaymentTransaction, error)
	UpdateStatus(ctx context.Context, providerRefID string, status entity.PaymentTransactionStatus, walletTransactionID *uint, rawNotification *string) error
}

type paymentRepositoryImpl struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepositoryImpl{db: db}
}

func (r *paymentRepositoryImpl) Create(ctx context.Context, payment *entity.PaymentTransaction) error {
	err := r.db.WithContext(ctx).Create(payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperror.ErrDuplicatedKey
		}
		return err
	}
	return nil
}

func (r *paymentRepositoryImpl) FindByProviderRefID(ctx context.Context, providerRefID string) (*entity.PaymentTransaction, error) {
	var payment entity.PaymentTransaction
	err := r.db.WithContext(ctx).Where("provider_ref_id = ?", providerRefID).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepositoryImpl) UpdateStatus(ctx context.Context, providerRefID string, status entity.PaymentTransactionStatus, walletTransactionID *uint, rawNotification *string) error {
	err := r.db.WithContext(ctx).
		Model(&entity.PaymentTransaction{}).
		Where("provider_ref_id = ?", providerRefID).
		Updates(map[string]any{
			"status":                status,
			"wallet_transaction_id": walletTransactionID,
			"raw_notification":      rawNotification,
		}).Error
	if err != nil {
		return err
	}
	return nil
}
