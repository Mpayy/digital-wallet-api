package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

func newTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupWalletUsecase(t *testing.T) (usecase.WalletUsecase, *mocks.MockWalletRepository, *mocks.MockTransactionRepository, *mocks.MockIdempotencyService) {
	walletRepo := mocks.NewMockWalletRepository(t)
	transactionRepo := mocks.NewMockTransactionRepository(t)
	idemService := mocks.NewMockIdempotencyService(t)
	log := newTestLogger()

	usecase := usecase.NewWalletUsecase(walletRepo, transactionRepo, idemService, log)
	t.Cleanup(func() {
		walletRepo.AssertExpectations(t)
		transactionRepo.AssertExpectations(t)
		idemService.AssertExpectations(t)
	})

	return usecase, walletRepo, transactionRepo, idemService
}

func TestWalletUsecase_CreateWallet(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)

	t.Run("success_create_wallet", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)

		walletRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID
		})).Return(nil)

		wallet, err := usecase.CreateWallet(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, userID, wallet.UserID)
		assert.Equal(t, int64(0), wallet.Balance)
	})

	t.Run("failed_create_wallet_duplicate", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)

		walletRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID
		})).Return(apperror.ErrDuplicatedKey)

		wallet, err := usecase.CreateWallet(ctx, userID)

		assert.ErrorIs(t, err, apperror.ErrUserHasWalletAlready)
		assert.Nil(t, wallet)
	})

	t.Run("failed_create_wallet_unexpected_error", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)
		dbErr := errors.New("unexpected error")

		walletRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID
		})).Return(dbErr)

		wallet, err := usecase.CreateWallet(ctx, userID)

		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, wallet)
	})
}

func TestWalletUsecase_GetWalletByUserID(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	wallet := &entity.Wallet{
		ID:      1,
		UserID:  userID,
		Balance: 100000,
	}

	t.Run("success_GetWalletByUserID", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		wallet, err := usecase.GetWalletByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, userID, wallet.UserID)
		assert.Equal(t, int64(100000), wallet.Balance)
	})

	t.Run("FindByUserID_not_found_auto_Create", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(nil, apperror.ErrRecordNotFound)
		walletRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID
		})).Return(nil)

		wallet, err := usecase.GetWalletByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, userID, wallet.UserID)
		assert.Equal(t, int64(0), wallet.Balance)
	})

	t.Run("failed_get_wallet_lazy_create_duplicate_key", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).
			Return(nil, apperror.ErrRecordNotFound)

		walletRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID && w.Balance == 0
		})).Return(apperror.ErrDuplicatedKey)

		result, err := usecase.GetWalletByUserID(ctx, userID)

		assert.ErrorIs(t, err, apperror.ErrUserHasWalletAlready)
		assert.Nil(t, result)
	})

	t.Run("unexpected_error_get_wallet_by_user_id", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)
		dbErr := errors.New("unexpected error")

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(nil, dbErr)

		wallet, err := usecase.GetWalletByUserID(ctx, userID)

		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, wallet)
	})
}

func TestWalletUsecase_TopUp(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	idemKey := "idemKey"
	topUpAmount := int64(5000)
	initialBalance := int64(0)
	expectedBalance := initialBalance + topUpAmount

	request := dto.TopUpRequest{
		Amount: topUpAmount,
	}

	t.Run("success_top_up", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService := setupWalletUsecase(t)

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).Return(true, "", nil)

		walletRepo.EXPECT().
			WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		walletRepo.EXPECT().LockByID(mock.Anything, wallet.ID).Return(wallet, nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID && w.Balance == expectedBalance
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tx *entity.Transaction) bool {
			return tx != nil &&
				tx.WalletID == wallet.ID &&
				tx.Type == entity.TxTypeTopup &&
				tx.Amount == request.Amount &&
				tx.BalanceBefore == initialBalance &&
				tx.BalanceAfter == expectedBalance &&
				tx.Status == entity.TxStatusSuccess
		})).Return(nil)

		idemService.EXPECT().Complete(mock.Anything, idemKey, mock.MatchedBy(func(res *dto.TopUpResponse) bool {
			return res != nil && res.WalletID == wallet.ID && res.BalanceAfter == expectedBalance
		})).Return(nil)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, wallet.ID, result.WalletID)
		assert.Equal(t, string(entity.TxTypeTopup), result.Type)
		assert.Equal(t, request.Amount, result.Amount)
		assert.Equal(t, initialBalance, result.BalanceBefore)
		assert.Equal(t, expectedBalance, result.BalanceAfter)
		assert.Equal(t, string(entity.TxStatusSuccess), result.Status)
	})

	t.Run("failed_invalid_amount", func(t *testing.T) {
		usecase, walletRepo, _, _ := setupWalletUsecase(t)
		invalidRequest := dto.TopUpRequest{
			Amount: 0,
		}

		result, err := usecase.TopUp(ctx, userID, invalidRequest, idemKey)
		assert.ErrorIs(t, err, apperror.ErrInvalidAmount)
		assert.Nil(t, result)

		walletRepo.AssertNotCalled(t, "FindByUserID")
		walletRepo.AssertNotCalled(t, "WithTx")
	})

	t.Run("failed_wallet_not_found", func(t *testing.T) {
		usecase, walletRepo, _, idemService := setupWalletUsecase(t)

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(nil, apperror.ErrRecordNotFound)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)
		assert.ErrorIs(t, err, apperror.ErrWalletNotFound)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Claim")
		walletRepo.AssertNotCalled(t, "WithTx")
	})

	t.Run("success_duplicate_request_idempotent", func(t *testing.T) {
		usecase, walletRepo, _, idemService := setupWalletUsecase(t)

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		validCachedJSON := `{
			"transaction_id": 1,
			"wallet_id": 1,
			"type": "TOPUP",
			"amount": 50000,
			"balance_before": 100000,
			"balance_after": 150000,
			"status": "SUCCESS"
		}`

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).Return(false, validCachedJSON, nil)

		var cached dto.TopUpResponse
		err := json.Unmarshal([]byte(validCachedJSON), &cached)
		assert.NoError(t, err)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)
		assert.NoError(t, err)
		assert.Equal(t, &cached, result)
		walletRepo.AssertNotCalled(t, "WithTx")
		idemService.AssertNotCalled(t, "MarkFailed")
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_top_up_claim_returns_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService := setupWalletUsecase(t)

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).
			Return(false, "", apperror.ErrIdempotencyKeyConflict)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)

		assert.ErrorIs(t, err, apperror.ErrIdempotencyKeyConflict)
		assert.Nil(t, result)

		walletRepo.AssertNotCalled(t, "WithTx")
		idemService.AssertNotCalled(t, "Complete")
		idemService.AssertNotCalled(t, "MarkFailed")
	})

	t.Run("failed_top_up_malformed_cached_json", func(t *testing.T) {
		usecase, walletRepo, _, idemService := setupWalletUsecase(t)

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		malformedJSON := "invalid-json-structure-{"
		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).
			Return(false, malformedJSON, nil)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal cached top up response")
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "WithTx")
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_tx_rollback_marks_idempotency_failed", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService := setupWalletUsecase(t)
		dbErr := errors.New("unexpected error")

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error) error {
				return fn(nil)
			})

		walletRepo.EXPECT().LockByID(mock.Anything, wallet.ID).Return(wallet, nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.Anything).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)

		transactionRepo.AssertNotCalled(t, "Create")
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("success_even_if_idempotency_complete_fails", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService := setupWalletUsecase(t)
		dbErr := errors.New("unexpected error")

		wallet := &entity.Wallet{
			ID:      1,
			UserID:  userID,
			Balance: initialBalance,
		}

		walletRepo.EXPECT().FindByUserID(mock.Anything, userID).Return(wallet, nil)

		idemService.EXPECT().Claim(mock.Anything, idemKey, userID, "TOPUP", request).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error) error {
				return fn(nil)
			})

		walletRepo.EXPECT().LockByID(mock.Anything, wallet.ID).Return(wallet, nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == userID && w.Balance == expectedBalance
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tx *entity.Transaction) bool {
			return tx != nil &&
				tx.WalletID == wallet.ID &&
				tx.Type == entity.TxTypeTopup &&
				tx.Amount == request.Amount &&
				tx.BalanceBefore == initialBalance &&
				tx.BalanceAfter == expectedBalance &&
				tx.Status == entity.TxStatusSuccess
		})).Return(nil)

		idemService.EXPECT().Complete(mock.Anything, idemKey, mock.MatchedBy(func(res *dto.TopUpResponse) bool {
			return res != nil && res.WalletID == wallet.ID && res.BalanceAfter == expectedBalance
		})).Return(dbErr)

		result, err := usecase.TopUp(ctx, userID, request, idemKey)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, wallet.ID, result.WalletID)
		assert.Equal(t, string(entity.TxTypeTopup), result.Type)
		assert.Equal(t, request.Amount, result.Amount)
		assert.Equal(t, initialBalance, result.BalanceBefore)
		assert.Equal(t, expectedBalance, result.BalanceAfter)
		assert.Equal(t, string(entity.TxStatusSuccess), result.Status)

		idemService.AssertNotCalled(t, "MarkFailed")
	})
}
