package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/hashutil"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/sirupsen/logrus"
)

type IdempotencyService interface {
	Claim(ctx context.Context, key string, userID uint, endpoint string, payload any) (claimed bool, cachedBody string, err error)
	Complete(ctx context.Context, key string, response any) error
	MarkFailed(ctx context.Context, key string) error
}

type idempotencyServiceImpl struct {
	log             *logrus.Logger
	idempotencyRepo repository.IdempotencyRepository
}

func NewIdempotencyService(log *logrus.Logger, idempotencyRepo repository.IdempotencyRepository) IdempotencyService {
	return &idempotencyServiceImpl{log: log, idempotencyRepo: idempotencyRepo}
}

func (s *idempotencyServiceImpl) Claim(ctx context.Context, key string, userID uint, endpoint string, payload any) (claimed bool, cachedBody string, err error) {
	if key == "" {
		return false, "", apperror.ErrMissingIdempotencyKey
	}

	reqHash, err := hashutil.HashPayload(payload)
	if err != nil {
		return false, "", err
	}

	record := &entity.IdempotencyKey{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: reqHash,
		Status:      entity.IdemStatusProcessing,
	}

	err = s.idempotencyRepo.Insert(ctx, record)
	if err == nil {
		return true, "", nil
	}

	if !errors.Is(err, apperror.ErrDuplicatedKey) {
		return false, "", fmt.Errorf("insert idempotency key: %w", err)
	}

	existing, findErr := s.idempotencyRepo.FindByKey(ctx, key)
	if findErr != nil {
		return false, "", fmt.Errorf("find idempotency key: %w", findErr)
	}

	if existing.RequestHash != reqHash {
		s.log.WithFields(logrus.Fields{
			"key": key, "user_id": userID, "endpoint": endpoint,
			"existing_hash": existing.RequestHash, "new_hash": reqHash,
		}).Warn("idempotency key reused with different payload")
		return false, "", apperror.ErrIdempotencyKeyConflict
	}

	switch existing.Status {
	case entity.IdemStatusCompleted:
		return false, existing.ResponseBody, nil
	case entity.IdemStatusProcessing:
		return false, "", apperror.ErrRequestInProgress
	default:
		return false, "", apperror.ErrPreviousAttemptFailed
	}
}

func (s *idempotencyServiceImpl) Complete(ctx context.Context, key string, response any) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal idempotency response: %w", err)
	}
	err = s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusCompleted, 201, string(responseJSON))
	if err != nil {
		return fmt.Errorf("update idempotency status: %w", err)
	}
	return nil
}

func (s *idempotencyServiceImpl) MarkFailed(ctx context.Context, key string) error {
	err := s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusFailed, 0, "")
	if err != nil {
		return fmt.Errorf("update idempotency status: %w", err)
	}
	return nil
}
