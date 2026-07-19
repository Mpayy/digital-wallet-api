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
	// logic: coba Insert (status PROCESSING) -> ErrDuplicatedKey? FindByKey
	//   -> beda request_hash? conflict -> status COMPLETED? return cached -> PROCESSING? request-in-progress

	Complete(ctx context.Context, key string, response any) error
	// logic: json.Marshal(response) -> UpdateStatus(COMPLETED, 201, responseJSON)

	MarkFailed(ctx context.Context, key string) error
	// logic: UpdateStatus(FAILED, 0, "")
}

type idempotencyServiceImpl struct {
	log             *logrus.Logger
	idempotencyRepo repository.IdempotencyRepository
}

func NewIdempotencyService(log *logrus.Logger, idempotencyRepo repository.IdempotencyRepository) IdempotencyService {
	return &idempotencyServiceImpl{log: log, idempotencyRepo: idempotencyRepo}
}

func (s *idempotencyServiceImpl) Claim(ctx context.Context, key string, userID uint, endpoint string, payload any) (claimed bool, cachedBody string, err error) {
	logger := s.log.WithFields(logrus.Fields{
		"key":      key,
		"user_id":  userID,
		"endpoint": endpoint,
	})
	logger.Debug("attempting to claim idempotency key")

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
		logger.Debug("idempotency key claimed successfully")
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
		logger.WithFields(logrus.Fields{
			"existing_hash": existing.RequestHash, "new_hash": reqHash,
		}).Warn("idempotency key reused with different payload")
		return false, "", apperror.ErrIdempotencyKeyConflict
	}

	switch existing.Status {
	case entity.IdemStatusCompleted:
		logger.Info("idempotency key already completed")
		return false, existing.ResponseBody, nil
	case entity.IdemStatusProcessing:
		return false, "", apperror.ErrRequestInProgress
	default:
		return false, "", apperror.ErrPreviousAttemptFailed
	}
}

func (s *idempotencyServiceImpl) Complete(ctx context.Context, key string, response any) error {
	logger := s.log.WithFields(logrus.Fields{
		"key": key,
	})
	logger.Debug("attempting to complete idempotency key")

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal idempotency response: %w", err)
	}
	err = s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusCompleted, 201, string(responseJSON))
	if err != nil {
		return fmt.Errorf("update idempotency status: %w", err)
	}

	logger.Debug("idempotency key completed successfully")
	return nil
}

func (s *idempotencyServiceImpl) MarkFailed(ctx context.Context, key string) error {
	logger := s.log.WithFields(logrus.Fields{
		"key": key,
	})
	logger.Debug("attempting to mark idempotency key as failed")

	err := s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusFailed, 0, "")
	if err != nil {
		return fmt.Errorf("update idempotency status: %w", err)
	}

	logger.Debug("idempotency key marked as failed successfully")
	return nil
}
