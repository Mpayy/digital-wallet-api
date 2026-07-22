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

func newTestLoggerTransfer() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func newDummyFromWallet() *entity.Wallet {
	return &entity.Wallet{
		ID:      101,
		UserID:  1,
		Balance: 10000,
	}
}

func newDummyToWallet() *entity.Wallet {
	return &entity.Wallet{
		ID:      102,
		UserID:  2,
		Balance: 10000,
	}
}

func setupTransferUsecase(t *testing.T) (usecase.TransferUsecase, *mocks.MockWalletRepository, *mocks.MockTransactionRepository, *mocks.MockIdempotencyService, *mocks.MockTransferRepository) {
	walletRepo := mocks.NewMockWalletRepository(t)
	transactionRepo := mocks.NewMockTransactionRepository(t)
	idemService := mocks.NewMockIdempotencyService(t)
	transferRepo := mocks.NewMockTransferRepository(t)
	log := newTestLoggerTransfer()

	usecase := usecase.NewTransferUsecase(transferRepo, walletRepo, idemService, transactionRepo, log)
	t.Cleanup(func() {
		walletRepo.AssertExpectations(t)
		transactionRepo.AssertExpectations(t)
		idemService.AssertExpectations(t)
		transferRepo.AssertExpectations(t)
	})

	return usecase, walletRepo, transactionRepo, idemService, transferRepo
}

func TestTransferUsecase_Transfer(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("unexpected error")
	idemKey := "idemKey"
	transferReq := dto.TransferRequest{
		ToUserID: 2,
		Amount:   5000,
		Note:     "test",
	}

	t.Run("failed_invalid_amount", func(t *testing.T) {
		usecase, walletRepo, _, _, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()

		invalidTransferReq := dto.TransferRequest{
			ToUserID: 2,
			Amount:   0,
			Note:     "test",
		}

		result, err := usecase.Transfer(ctx, fromWallet.UserID, invalidTransferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrInvalidAmount)
		walletRepo.AssertNotCalled(t, "FindByUserID")
	})

	t.Run("failed_sender_wallet_not_found", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(nil, apperror.ErrRecordNotFound)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrWalletNotFound)
		idemService.AssertNotCalled(t, "Claim")
	})

	t.Run("failed_sender_wallet_unexpected_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(nil, dbErr)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
		idemService.AssertNotCalled(t, "Claim")
	})

	t.Run("failed_recipient_wallet_not_found", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(nil, apperror.ErrRecordNotFound)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrRecipientNotFound)
		idemService.AssertNotCalled(t, "Claim")
	})

	t.Run("failed_recipient_wallet_unexpected_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(nil, dbErr)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
		idemService.AssertNotCalled(t, "Claim")
	})

	t.Run("failed_self_transfer", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyFromWallet()

		selfTransferReq := dto.TransferRequest{
			ToUserID: fromWallet.UserID,
			Amount:   5000,
			Note:     "test",
		}

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, selfTransferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrSelfTransferNotAllowed)
		idemService.AssertNotCalled(t, "Claim")
	})

	t.Run("failed_claim_returns_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(false, "", apperror.ErrIdempotencyKeyConflict)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrIdempotencyKeyConflict)
		walletRepo.AssertNotCalled(t, "WithTx")
	})

	t.Run("success_replayed_from_cache", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		validCachedJSON := `{
			"transfer_id": 1,
			"transaction_id": 1,
			"type": "TRANSFER",
			"amount": 50000,
			"balance_before": 100000,
			"balance_after": 150000,
			"counterparty_user_id": 2,
			"status": "SUCCESS"
		}`

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(false, validCachedJSON, nil)

		var cached dto.TransferResponse
		err := json.Unmarshal([]byte(validCachedJSON), &cached)
		assert.NoError(t, err)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.NoError(t, err)
		assert.Equal(t, &cached, result)
		walletRepo.AssertNotCalled(t, "WithTx")
	})

	t.Run("failed_cached_body_malformed", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		malformedJSON := "invalid-json-structure-{"

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(false, malformedJSON, nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal cached transfer response")
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "WithTx")
	})

	t.Run("success_transfer_sender_id_larger_than_recipient_id", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := &entity.Wallet{
			ID:      105,
			UserID:  5,
			Balance: 10000,
		}

		toWallet := &entity.Wallet{
			ID:      103,
			UserID:  3,
			Balance: 10000,
		}

		largeRecipientIDTransferReq := dto.TransferRequest{
			ToUserID: toWallet.UserID,
			Amount:   5000,
			Note:     "test_sender_id_larger_than_recipient_id",
		}

		fromBalanceBefore := fromWallet.Balance
		toBalanceBefore := toWallet.Balance
		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", largeRecipientIDTransferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callRecipient.Call, callSender.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == fromWallet.UserID &&
				w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == toWallet.UserID &&
				w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == largeRecipientIDTransferReq.Amount &&
				tf.Note == largeRecipientIDTransferReq.Note
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(outTx *entity.Transaction) bool {
			return outTx != nil &&
				outTx.WalletID == fromWallet.ID &&
				outTx.Type == entity.TxTypeTransferOut &&
				outTx.Amount == largeRecipientIDTransferReq.Amount &&
				outTx.BalanceBefore == fromBalanceBefore &&
				outTx.BalanceAfter == expectedFromBalance &&
				outTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(inTx *entity.Transaction) bool {
			return inTx != nil &&
				inTx.WalletID == toWallet.ID &&
				inTx.Type == entity.TxTypeTransferIn &&
				inTx.Amount == largeRecipientIDTransferReq.Amount &&
				inTx.BalanceBefore == toBalanceBefore &&
				inTx.BalanceAfter == expectedToBalance &&
				inTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		idemService.EXPECT().Complete(mock.Anything, idemKey, mock.Anything).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, largeRecipientIDTransferReq, idemKey)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, expectedFromBalance, fromWallet.Balance)
		assert.Equal(t, expectedToBalance, toWallet.Balance)
	})

	t.Run("failed_lock_first_wallet_not_found", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(nil, apperror.ErrRecordNotFound)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, apperror.ErrWalletNotFound)
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "LockByID", mock.Anything, toWallet.ID)
	})

	t.Run("failed_lock_first_wallet_unexpected_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(nil, dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "LockByID", mock.Anything, toWallet.ID)
	})

	t.Run("failed_lock_second_wallet_not_found", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(nil, apperror.ErrRecordNotFound)

		mock.InOrder(callSender.Call, callRecipient.Call)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, apperror.ErrWalletNotFound)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_lock_second_wallet_unexpected_error", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(nil, dbErr)

		mock.InOrder(callSender.Call, callRecipient.Call)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_insufficient_balance", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		invalidtransferReq := dto.TransferRequest{
			ToUserID: 2,
			Amount:   10001,
			Note:     "test_failed_insufficient_balance",
		}

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", invalidtransferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, invalidtransferReq, idemKey)
		assert.ErrorIs(t, err, apperror.ErrInsufficientBalance)
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "Save", mock.Anything, fromWallet)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_save_sender_wallet", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		expectedFromBalance := int64(5000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == fromWallet.UserID && w.Balance == expectedFromBalance
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		walletRepo.AssertNotCalled(t, "Save", mock.Anything, toWallet)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_save_recipient_wallet", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == fromWallet.UserID && w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == toWallet.UserID && w.Balance == expectedToBalance
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_create_transfer_record", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == fromWallet.UserID && w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == toWallet.UserID && w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == transferReq.Amount &&
				tf.Note == transferReq.Note
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
		transactionRepo.AssertNotCalled(t, "Create")
	})

	t.Run("failed_create_outgoing_transaction", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		fromBalanceBefore := fromWallet.Balance
		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == fromWallet.UserID && w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == toWallet.UserID && w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == transferReq.Amount &&
				tf.Note == transferReq.Note
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tx *entity.Transaction) bool {
			return tx != nil &&
				tx.WalletID == fromWallet.ID &&
				tx.Type == entity.TxTypeTransferOut &&
				tx.Amount == transferReq.Amount &&
				tx.BalanceBefore == fromBalanceBefore &&
				tx.BalanceAfter == expectedFromBalance &&
				tx.Status == entity.TxStatusSuccess
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("failed_create_incoming_transaction", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		fromBalanceBefore := fromWallet.Balance
		toBalanceBefore := toWallet.Balance
		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		callSender := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		callRecipient := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(callSender.Call, callRecipient.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == fromWallet.UserID && w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil && w.UserID == toWallet.UserID && w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == transferReq.Amount &&
				tf.Note == transferReq.Note
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tx *entity.Transaction) bool {
			return tx != nil &&
				tx.WalletID == fromWallet.ID &&
				tx.Type == entity.TxTypeTransferOut &&
				tx.Amount == transferReq.Amount &&
				tx.BalanceBefore == fromBalanceBefore &&
				tx.BalanceAfter == expectedFromBalance &&
				tx.Status == entity.TxStatusSuccess
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tx *entity.Transaction) bool {
			return tx != nil &&
				tx.WalletID == toWallet.ID &&
				tx.Type == entity.TxTypeTransferIn &&
				tx.Amount == transferReq.Amount &&
				tx.BalanceBefore == toBalanceBefore &&
				tx.BalanceAfter == expectedToBalance &&
				tx.Status == entity.TxStatusSuccess
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("success_transfer", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		fromBalanceBefore := fromWallet.Balance
		toBalanceBefore := toWallet.Balance
		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		call1 := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		call2 := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(call1.Call, call2.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == fromWallet.UserID &&
				w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == toWallet.UserID &&
				w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == transferReq.Amount &&
				tf.Note == transferReq.Note
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(outTx *entity.Transaction) bool {
			return outTx != nil &&
				outTx.WalletID == fromWallet.ID &&
				outTx.Type == entity.TxTypeTransferOut &&
				outTx.Amount == transferReq.Amount &&
				outTx.BalanceBefore == fromBalanceBefore &&
				outTx.BalanceAfter == expectedFromBalance &&
				outTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(inTx *entity.Transaction) bool {
			return inTx != nil &&
				inTx.WalletID == toWallet.ID &&
				inTx.Type == entity.TxTypeTransferIn &&
				inTx.Amount == transferReq.Amount &&
				inTx.BalanceBefore == toBalanceBefore &&
				inTx.BalanceAfter == expectedToBalance &&
				inTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		idemService.EXPECT().Complete(mock.Anything, idemKey, mock.Anything).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, string(entity.TxTypeTransferOut), result.Type)
		assert.Equal(t, transferReq.Amount, result.Amount)
		assert.Equal(t, fromBalanceBefore, result.BalanceBefore)
		assert.Equal(t, expectedFromBalance, result.BalanceAfter)
		assert.Equal(t, toWallet.UserID, result.CounterPartyUserID)
		assert.Equal(t, string(entity.TxStatusSuccess), result.Status)
	})

	t.Run("failed_tx_rollback_marks_idempotency_failed", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		expectedFromBalance := int64(5000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		call1 := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		call2 := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(call1.Call, call2.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == fromWallet.UserID &&
				w.Balance == expectedFromBalance
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(nil)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})

	t.Run("success_even_if_idempotency_complete_fails", func(t *testing.T) {
		usecase, walletRepo, transactionRepo, idemService, transferRepo := setupTransferUsecase(t)

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		fromBalanceBefore := fromWallet.Balance
		toBalanceBefore := toWallet.Balance
		expectedFromBalance := int64(5000)
		expectedToBalance := int64(15000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		call1 := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		call2 := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(call1.Call, call2.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == fromWallet.UserID &&
				w.Balance == expectedFromBalance
		})).Return(nil)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == toWallet.UserID &&
				w.Balance == expectedToBalance
		})).Return(nil)

		transferRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tf *entity.Transfer) bool {
			return tf != nil &&
				tf.FromWalletID == fromWallet.ID &&
				tf.ToWalletID == toWallet.ID &&
				tf.Amount == transferReq.Amount &&
				tf.Note == transferReq.Note
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(outTx *entity.Transaction) bool {
			return outTx != nil &&
				outTx.WalletID == fromWallet.ID &&
				outTx.Type == entity.TxTypeTransferOut &&
				outTx.Amount == transferReq.Amount &&
				outTx.BalanceBefore == fromBalanceBefore &&
				outTx.BalanceAfter == expectedFromBalance &&
				outTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		transactionRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(inTx *entity.Transaction) bool {
			return inTx != nil &&
				inTx.WalletID == toWallet.ID &&
				inTx.Type == entity.TxTypeTransferIn &&
				inTx.Amount == transferReq.Amount &&
				inTx.BalanceBefore == toBalanceBefore &&
				inTx.BalanceAfter == expectedToBalance &&
				inTx.Status == entity.TxStatusSuccess
		})).Return(nil)

		idemService.EXPECT().Complete(mock.Anything, idemKey, mock.Anything).Return(dbErr)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, result.Type, string(entity.TxTypeTransferOut))
		assert.Equal(t, result.Amount, transferReq.Amount)
		assert.Equal(t, result.BalanceBefore, fromBalanceBefore)
		assert.Equal(t, result.BalanceAfter, expectedFromBalance)
		assert.Equal(t, result.CounterPartyUserID, toWallet.UserID)
		assert.Equal(t, result.Status, string(entity.TxStatusSuccess))
	})

	t.Run("failed_tx_rollback_and_mark_failed_also_fails", func(t *testing.T) {
		usecase, walletRepo, _, idemService, _ := setupTransferUsecase(t)
		dbIdempotencyErr := errors.New("unexpected mark failed error")

		fromWallet := newDummyFromWallet()
		toWallet := newDummyToWallet()

		expectedFromBalance := int64(5000)

		walletRepo.EXPECT().FindByUserID(ctx, fromWallet.UserID).Return(fromWallet, nil)
		walletRepo.EXPECT().FindByUserID(ctx, toWallet.UserID).Return(toWallet, nil)
		idemService.EXPECT().Claim(ctx, idemKey, fromWallet.UserID, "TRANSFER", transferReq).Return(true, "", nil)

		walletRepo.EXPECT().WithTx(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(*gorm.DB) error) error {
				return fn(nil)
			})

		call1 := walletRepo.EXPECT().LockByID(mock.Anything, fromWallet.ID).Return(fromWallet, nil)
		call2 := walletRepo.EXPECT().LockByID(mock.Anything, toWallet.ID).Return(toWallet, nil)

		mock.InOrder(call1.Call, call2.Call)

		walletRepo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(w *entity.Wallet) bool {
			return w != nil &&
				w.UserID == fromWallet.UserID &&
				w.Balance == expectedFromBalance
		})).Return(dbErr)

		idemService.EXPECT().MarkFailed(mock.Anything, idemKey).Return(dbIdempotencyErr)

		result, err := usecase.Transfer(ctx, fromWallet.UserID, transferReq, idemKey)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, result)
		idemService.AssertNotCalled(t, "Complete")
	})
}
