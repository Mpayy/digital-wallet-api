package usecase_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestLoggerTransaction() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupTransactionUseCase(t *testing.T) (usecase.TransactionUsecase, *mocks.MockTransactionRepository, *mocks.MockWalletUsecase) {
	transactionRepo := mocks.NewMockTransactionRepository(t)
	walletUsecase := mocks.NewMockWalletUsecase(t)
	log := newTestLoggerTransaction()

	usecase := usecase.NewTransactionUsecase(transactionRepo, walletUsecase, log)
	t.Cleanup(func() {
		transactionRepo.AssertExpectations(t)
		walletUsecase.AssertExpectations(t)
	})

	return usecase, transactionRepo, walletUsecase
}

func TestTransactionUsecase_GetTransactionHistory(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	walletID := uint(10)
	wallet := &dto.WalletResponse{
		ID:     walletID,
		UserID: userID,
	}
	dbErr := errors.New("unexpected error")

	t.Run("success_get_history", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		filter := dto.TransactionFilter{Page: 1, Limit: 10}
		now := time.Now()

		transactions := []entity.Transaction{
			{
				ID:            1,
				WalletID:      walletID,
				Type:          entity.TxTypeTopup,
				Amount:        100000,
				BalanceBefore: 0,
				BalanceAfter:  100000,
				Status:        entity.TxStatusSuccess,
				CreatedAt:     now,
			},
		}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, filter).Return(transactions, int64(25), nil)

		result, err := usecase.GetTransactionHistory(ctx, userID, filter)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Data, 1)

		assert.Equal(t, uint(1), result.Data[0].TransactionID)
		assert.Equal(t, string(entity.TxTypeTopup), result.Data[0].Type)
		assert.Equal(t, int64(100000), result.Data[0].Amount)
		assert.Equal(t, string(entity.TxStatusSuccess), result.Data[0].Status)

		assert.Equal(t, 1, result.Meta.Page)
		assert.Equal(t, 10, result.Meta.Limit)
		assert.Equal(t, int64(25), result.Meta.Total)
		assert.Equal(t, int64(3), result.Meta.TotalPages)
	})

	t.Run("success_total_pages_exact_division", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		filter := dto.TransactionFilter{Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, filter).Return([]entity.Transaction{}, int64(20), nil)

		result, err := usecase.GetTransactionHistory(ctx, userID, filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(20), result.Meta.Total)
		assert.Equal(t, int64(2), result.Meta.TotalPages)
	})

	t.Run("success_empty_result", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		filter := dto.TransactionFilter{Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)
		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, filter).Return([]entity.Transaction{}, int64(0), nil)

		result, err := usecase.GetTransactionHistory(ctx, userID, filter)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		assert.NotNil(t, result.Data)
		assert.Equal(t, []dto.TransactionResponse{}, result.Data)

		assert.Equal(t, int64(0), result.Meta.Total)
		assert.Equal(t, int64(0), result.Meta.TotalPages)
	})

	t.Run("success_default_page_and_limit_applied", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		inputFilter := dto.TransactionFilter{Page: 0, Limit: 0}
		expectedFilter := dto.TransactionFilter{Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, expectedFilter).Return([]entity.Transaction{}, int64(0), nil)

		result, err := usecase.GetTransactionHistory(ctx, userID, inputFilter)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.Meta.Page)
		assert.Equal(t, 10, result.Meta.Limit)
	})

	t.Run("failed_get_wallet_error", func(t *testing.T) {
		usecase, _, walletUsecase := setupTransactionUseCase(t)

		filter := dto.TransactionFilter{Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(nil, dbErr)

		result, err := usecase.GetTransactionHistory(ctx, userID, filter)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_find_transactions_error", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		filter := dto.TransactionFilter{Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)
		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, filter).Return(nil, int64(0), dbErr)

		res, err := usecase.GetTransactionHistory(ctx, userID, filter)

		assert.Nil(t, res)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("success_start_date_without_end_date_defaults_to_today", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		inputFilter := dto.TransactionFilter{StartDate: "2026-01-01", Page: 1, Limit: 10}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)
		transactionRepo.EXPECT().FindByWalletID(mock.Anything, walletID, mock.MatchedBy(func(f dto.TransactionFilter) bool {
			_, err := time.Parse("2006-01-02", f.EndDate)
			return f.EndDate != "" && err == nil
		})).Return([]entity.Transaction{}, int64(0), nil)

		_, err := usecase.GetTransactionHistory(ctx, userID, inputFilter)
		assert.NoError(t, err)
	})
}

func TestTransactionUsecase_GetTransactionDetail(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	walletID := uint(10)
	transactionID := uint(100)

	wallet := &dto.WalletResponse{
		ID:     walletID,
		UserID: userID,
	}

	transaction := &entity.Transaction{
		ID:            transactionID,
		WalletID:      walletID,
		Type:          entity.TxTypeTransferOut,
		Amount:        50000,
		BalanceBefore: 100000,
		BalanceAfter:  50000,
		Status:        entity.TxStatusSuccess,
		CreatedAt:     time.Now(),
	}

	dbErr := errors.New("unexpected error")

	t.Run("success_get_detail", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByID(mock.Anything, transactionID).Return(transaction, nil)

		result, err := usecase.GetTransactionDetail(ctx, userID, transactionID)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		assert.Equal(t, transaction.ID, result.TransactionID)
		assert.Equal(t, string(transaction.Type), result.Type)
		assert.Equal(t, transaction.Amount, result.Amount)
		assert.Equal(t, transaction.BalanceBefore, result.BalanceBefore)
		assert.Equal(t, transaction.BalanceAfter, result.BalanceAfter)
		assert.Equal(t, string(transaction.Status), result.Status)
		assert.Equal(t, transaction.CreatedAt, result.CreatedAt)
	})

	t.Run("failed_get_wallet_error", func(t *testing.T) {
		usecase, _, walletUsecase := setupTransactionUseCase(t)

		walletErr := errors.New("failed to fetch user wallet")

		walletUsecase.EXPECT().
			GetWalletByUserID(mock.Anything, userID).
			Return(nil, walletErr)

		result, err := usecase.GetTransactionDetail(ctx, userID, transactionID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, walletErr)
	})

	t.Run("failed_transaction_not_found", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByID(mock.Anything, transactionID).Return(nil, apperror.ErrRecordNotFound)

		result, err := usecase.GetTransactionDetail(ctx, userID, transactionID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrTransactionNotFound)
	})

	t.Run("failed_find_transaction_unexpected_error", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByID(mock.Anything, transactionID).Return(nil, dbErr)

		result, err := usecase.GetTransactionDetail(ctx, userID, transactionID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failed_transaction_belongs_to_other_wallet", func(t *testing.T) {
		usecase, transactionRepo, walletUsecase := setupTransactionUseCase(t)

		otherWalletTx := &entity.Transaction{
			ID:       transactionID,
			WalletID: uint(99),
		}

		walletUsecase.EXPECT().GetWalletByUserID(mock.Anything, userID).Return(wallet, nil)

		transactionRepo.EXPECT().FindByID(mock.Anything, transactionID).Return(otherWalletTx, nil)

		result, err := usecase.GetTransactionDetail(ctx, userID, transactionID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperror.ErrTransactionNotFound)
	})
}
