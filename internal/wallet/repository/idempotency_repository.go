package repository

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"gorm.io/gorm"
)

type IdempotencyRepository interface {
	Insert(ctx context.Context, record *entity.IdempotencyKey) error
	// INSERT klaim key baru — andalkan UNIQUE constraint kolom `key` utk deteksi race

	FindByKey(ctx context.Context, key string) (*entity.IdempotencyKey, error)
	// ambil record existing utk cek request_hash & status

	UpdateStatus(ctx context.Context, key string, status entity.IdempotencyStatus, responseStatus int, responseBody string) error
	// update jadi COMPLETED (simpan cached response) atau FAILED
}

type idempotencyRepositoryImpl struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) IdempotencyRepository {
	return &idempotencyRepositoryImpl{db: db}
}

func (r *idempotencyRepositoryImpl) Insert(ctx context.Context, record *entity.IdempotencyKey) error {
	err := r.db.WithContext(ctx).Create(record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperror.ErrDuplicatedKey
		}

		return err
	}

	return nil
}

func (r *idempotencyRepositoryImpl) FindByKey(ctx context.Context, key string) (*entity.IdempotencyKey, error) {
	var record entity.IdempotencyKey
	err := r.db.WithContext(ctx).Where("idem_key = ?", key).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrRecordNotFound
		}
		return nil, err
	}

	return &record, nil
}

func (r *idempotencyRepositoryImpl) UpdateStatus(ctx context.Context, key string, status entity.IdempotencyStatus, responseStatus int, responseBody string) error {
	err := r.db.WithContext(ctx).Model(&entity.IdempotencyKey{}).
		Where("idem_key = ?", key).
		Updates(map[string]any{
			"status":          status,
			"response_status": responseStatus,
			"response_body":   responseBody,
		}).Error
	if err != nil {
		return err
	}

	return nil
}
