package usecase

import (
	"context"
	"encoding/json"
	"errors"

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
	s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint, "payload": payload}).Debug("idem:Claiming request")

	if key == "" {
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint, "payload": payload}).Warn("idem:Missing idempotency key")
		return false, "", apperror.ErrMissingIdempotencyKey
	}

	reqHash, err := hashutil.HashPayload(payload)
	if err != nil {
		s.log.WithFields(logrus.Fields{"key": key, "error": err}).Error("idem:HashPayload failed")
		return false, "", apperror.ErrInternalServer
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
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint, "payload": payload}).Info("idem:Request claimed")
		return true, "", nil
	}

	if !errors.Is(err, apperror.ErrDuplicatedKey) {
		s.log.WithFields(logrus.Fields{"key": key, "error": err}).Warn("idem:Insert failed (not duplicated)")
		return false, "", apperror.ErrInternalServer
	}

	existing, findErr := s.idempotencyRepo.FindByKey(ctx, key)
	if findErr != nil {
		if errors.Is(findErr, apperror.ErrRecordNotFound) {
			s.log.WithFields(logrus.Fields{"key": key, "error": findErr}).Warn("idem:FindByKey failed")
			return false, "", apperror.ErrMissingIdempotencyKey
		}
		s.log.WithFields(logrus.Fields{"key": key, "error": findErr}).Error("idem:FindByKey failed")
		return false, "", apperror.ErrInternalServer
	}

	if existing.RequestHash != reqHash {
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "existing_hash": existing.RequestHash, "incoming_hash": reqHash}).Warn("idem:request hash not match")
		return false, "", apperror.ErrIdempotencyKeyConflict
	}

	switch existing.Status {
	case entity.IdemStatusCompleted:
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint}).Info("idem: Duplicate request detected, returning cached response")
		return false, existing.ResponseBody, nil
	case entity.IdemStatusProcessing:
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint}).Warn("idem:Request already in progress")
		return false, "", apperror.ErrRequestInProgress
	default:
		s.log.WithFields(logrus.Fields{"key": key, "user_id": userID, "endpoint": endpoint}).Warn("idem:Previous attempt failed")
		return false, "", apperror.ErrPreviousAttemptFailed
	}
}

func (s *idempotencyServiceImpl) Complete(ctx context.Context, key string, response any) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		s.log.WithFields(logrus.Fields{"key": key, "error": err}).Error("idem:Marshal failed")
		return apperror.ErrInternalServer
	}
	err = s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusCompleted, 201, string(responseJSON))
	if err != nil {
		s.log.WithFields(logrus.Fields{"key": key, "error": err}).Error("idem:UpdateStatus failed")
		return apperror.ErrInternalServer
	}
	return nil
}

func (s *idempotencyServiceImpl) MarkFailed(ctx context.Context, key string) error {
	err := s.idempotencyRepo.UpdateStatus(ctx, key, entity.IdemStatusFailed, 0, "")
	if err != nil {
		s.log.WithFields(logrus.Fields{"key": key, "error": err}).Error("idem:UpdateStatus failed")
		return apperror.ErrInternalServer
	}
	return nil
}
