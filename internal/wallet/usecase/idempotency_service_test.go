package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/hashutil"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestLoggerIdempotency() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupIdempotencyService(t *testing.T) (usecase.IdempotencyService, *mocks.MockIdempotencyRepository) {
	idemRepo := mocks.NewMockIdempotencyRepository(t)
	log := newTestLoggerIdempotency()

	usecase := usecase.NewIdempotencyService(log, idemRepo)
	t.Cleanup(func() {
		idemRepo.AssertExpectations(t)
	})

	return usecase, idemRepo
}

func TestIdempotencyService_Claim(t *testing.T) {
	ctx := context.Background()
	idemKey := "idem-key-123"
	payload := map[string]any{
		"to_user_id": 2,
		"amount":     5000,
	}
	userID := uint(1)
	endpoint := "TOPUP OR TRANSFER"
	dbErr := errors.New("unexpected error")

	t.Run("failed_empty_key", func(t *testing.T) {
		usecase, _ := setupIdempotencyService(t)

		claimed, cache, err := usecase.Claim(ctx, "", userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, apperror.ErrMissingIdempotencyKey)
	})

	t.Run("failed_hash_payload_error", func(t *testing.T) {
		usecase, _ := setupIdempotencyService(t)

		malformedPayload := map[string]any{"x": make(chan int)}

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, malformedPayload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.Contains(t, err.Error(), "hash payload failed")
	})

	t.Run("success_new_key_claimed", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(nil)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.True(t, claimed)
		assert.Empty(t, cache)
		assert.NoError(t, err)
	})

	t.Run("failed_insert_unexpected_error", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(dbErr)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_find_by_key_after_duplicate", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(apperror.ErrDuplicatedKey)

		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(nil, dbErr)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_hash_mismatch_conflict", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		existingRecord := &entity.IdempotencyKey{
			Key:         idemKey,
			UserID:      userID,
			Endpoint:    endpoint,
			RequestHash: "different_hash_from_previous_request_10000",
			Status:      entity.IdemStatusProcessing,
		}

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(apperror.ErrDuplicatedKey)

		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(existingRecord, nil)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, apperror.ErrIdempotencyKeyConflict)
	})

	t.Run("success_replay_completed", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		expectedResponseBody := `{"status":"success","message":"completed"}`
		expectedHash, err := hashutil.HashPayload(payload)
		assert.NoError(t, err)

		record := &entity.IdempotencyKey{
			Key:          idemKey,
			UserID:       userID,
			Endpoint:     endpoint,
			RequestHash:  expectedHash,
			ResponseBody: expectedResponseBody,
			Status:       entity.IdemStatusCompleted,
		}

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(apperror.ErrDuplicatedKey)

		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(record, nil)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Equal(t, expectedResponseBody, cache)
		assert.NoError(t, err)
	})

	t.Run("failed_request_in_progress", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		expectedHash, err := hashutil.HashPayload(payload)
		assert.NoError(t, err)

		record := &entity.IdempotencyKey{
			Key:          idemKey,
			UserID:       userID,
			Endpoint:     endpoint,
			RequestHash:  expectedHash,
			ResponseBody: "",
			Status:       entity.IdemStatusProcessing,
		}

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(apperror.ErrDuplicatedKey)

		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(record, nil)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, apperror.ErrRequestInProgress)
	})

	t.Run("failed_previous_attempt_failed", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		expectedHash, err := hashutil.HashPayload(payload)
		assert.NoError(t, err)

		record := &entity.IdempotencyKey{
			Key:          idemKey,
			UserID:       userID,
			Endpoint:     endpoint,
			RequestHash:  expectedHash,
			ResponseBody: "",
			Status:       entity.IdemStatusFailed,
		}

		idemRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(record *entity.IdempotencyKey) bool {
			return record.Key == idemKey &&
				record.UserID == userID &&
				record.Endpoint == endpoint &&
				record.Status == entity.IdemStatusProcessing
		})).Return(apperror.ErrDuplicatedKey)

		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(record, nil)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.ErrorIs(t, err, apperror.ErrPreviousAttemptFailed)
	})

	t.Run("failed_find_by_key_returns_record_not_found_is_treated_as_internal_error", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		idemRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(apperror.ErrDuplicatedKey)
		idemRepo.EXPECT().FindByKey(mock.Anything, idemKey).Return(nil, apperror.ErrRecordNotFound)

		claimed, cache, err := usecase.Claim(ctx, idemKey, userID, endpoint, payload)
		assert.False(t, claimed)
		assert.Empty(t, cache)
		assert.NotErrorIs(t, err, apperror.ErrMissingIdempotencyKey)
	})
}

func TestIdempotencyService_Complete(t *testing.T) {
	ctx := context.Background()
	idemKey := "idem-key-123"
	dbErr := errors.New("unexpected error")

	t.Run("failed_marshal_response_error", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		malformedResponse := map[string]any{"x": make(chan int)}

		err := usecase.Complete(ctx, idemKey, malformedResponse)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marshal idempotency response")
		idemRepo.AssertNotCalled(t, "UpdateStatus")
	})

	t.Run("failed_update_status_error", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		response := map[string]string{"status": "success", "message": "completed"}

		idemRepo.EXPECT().UpdateStatus(
			mock.Anything,
			idemKey,
			entity.IdemStatusCompleted,
			201,
			mock.MatchedBy(func(r string) bool {
				var res map[string]string
				_ = json.Unmarshal([]byte(r), &res)
				return res["status"] == "success" && res["message"] == "completed"
			}),
		).Return(dbErr)

		err := usecase.Complete(ctx, idemKey, response)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_complete", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)

		response := map[string]string{"status": "success", "message": "completed"}

		idemRepo.EXPECT().UpdateStatus(
			mock.Anything,
			idemKey,
			entity.IdemStatusCompleted,
			201,
			mock.MatchedBy(func(r string) bool {
				var res map[string]string
				_ = json.Unmarshal([]byte(r), &res)
				return res["status"] == "success" && res["message"] == "completed"
			}),
		).Return(nil)

		err := usecase.Complete(ctx, idemKey, response)

		assert.NoError(t, err)
	})
}

func TestIdempotencyService_MarkFailed(t *testing.T) {
	ctx := context.Background()

	t.Run("failed_update_status_error", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)
		dbErr := errors.New("unexpected error")
		idemKey := "idem-key-123"

		idemRepo.EXPECT().UpdateStatus(
			mock.Anything,
			idemKey,
			entity.IdemStatusFailed,
			0,
			"",
		).Return(dbErr)

		err := usecase.MarkFailed(ctx, idemKey)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_mark_failed", func(t *testing.T) {
		usecase, idemRepo := setupIdempotencyService(t)
		idemKey := "idem-key-123"

		idemRepo.EXPECT().UpdateStatus(
			mock.Anything,
			idemKey,
			entity.IdemStatusFailed,
			0,
			"",
		).Return(nil)

		err := usecase.MarkFailed(ctx, idemKey)
		assert.NoError(t, err)
		idemRepo.AssertCalled(t, "UpdateStatus", mock.Anything, idemKey, entity.IdemStatusFailed, 0, "")
	})
}
