package usecase_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	"github.com/Mpayy/digital-wallet-api/internal/payment/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	walletdto "github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestLoggerWithdrawal() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupWithdrawalUsecase(t *testing.T) (usecase.WithdrawalUsecase, *mocks.MockWalletWithdrawer, *mocks.MockPaymentDisburser, *mocks.MockPaymentRepository) {
	paymentRepo := mocks.NewMockPaymentRepository(t)
	walletWithdrawer := mocks.NewMockWalletWithdrawer(t)
	paymentDisburser := mocks.NewMockPaymentDisburser(t)
	log := newTestLoggerWithdrawal()

	usecase := usecase.NewWithdrawalUsecase(paymentRepo, paymentDisburser, walletWithdrawer, log)
	t.Cleanup(func() {
		paymentRepo.AssertExpectations(t)
		paymentDisburser.AssertExpectations(t)
		walletWithdrawer.AssertExpectations(t)
	})

	return usecase, walletWithdrawer, paymentDisburser, paymentRepo
}

func TestWithdrawalUsecase_CreateWithdrawal(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	idemKey := "idem-key-123"
	req := dto.WithdrawalRequest{
		Amount:            100000,
		ChannelCode:       "ID_DANA",
		AccountNumber:     "08123456789",
		AccountHolderName: "John Doe",
	}

	dummyWithdrawRes := &walletdto.WithdrawResponse{
		TransactionID: 101,
		WalletID:      1,
		Amount:        100000,
		BalanceBefore: 500000,
		BalanceAfter:  400000,
		CreatedAt:     time.Now(),
	}

	dummyPayoutRes := &gateway.PayoutResult{
		ProviderRefID: "disb-12345",
		Status:        "PENDING",
	}

	t.Run("Success - Full Flow", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		walletWithdrawer.On("Withdraw", ctx, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", ctx, gateway.PayoutRequest{
			ReferenceID:       "WITHDRAWAL-101",
			ChannelCode:       req.ChannelCode,
			AccountNumber:     req.AccountNumber,
			AccountHolderName: req.AccountHolderName,
			Amount:            req.Amount,
		}).Return(dummyPayoutRes, nil)

		paymentRepo.On("Create", ctx, mock.MatchedBy(func(record *entity.PaymentTransaction) bool {
			return record.Provider == "XENDIT" && record.ProviderRefID == "disb-12345" && *record.WalletTransactionID == uint(101)
		})).Return(nil)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, uint(101), res.TransactionID)
		assert.Equal(t, "disb-12345", res.ReferenceID)
		walletWithdrawer.AssertExpectations(t)
		paymentDisburser.AssertExpectations(t)
		paymentRepo.AssertExpectations(t)
	})

	t.Run("Fail - Wallet Withdraw Error", func(t *testing.T) {
		usecase, walletWithdrawer, _, _ := setupWithdrawalUsecase(t)

		withDrawErr := errors.New("Withdraw Error")
		walletWithdrawer.On("Withdraw", ctx, userID, req.Amount, idemKey).Return(nil, withDrawErr)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, withDrawErr)
	})

	t.Run("Fail - Gateway Error (Reversal Succeeded)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, _ := setupWithdrawalUsecase(t)

		gatewayErr := errors.New("xendit connection timeout")
		walletWithdrawer.On("Withdraw", ctx, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", ctx, mock.Anything).Return(nil, gatewayErr)
		walletWithdrawer.On("ReverseWithdrawal", ctx, uint(101), "Gateway error: xendit connection timeout").Return(nil)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, gatewayErr)
	})

	t.Run("Fail - Gateway Error AND Reversal Failed (Critical)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, _ := setupWithdrawalUsecase(t)

		gatewayErr := errors.New("xendit connection timeout")
		dbErr := errors.New("db lock timeout")

		walletWithdrawer.On("Withdraw", ctx, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", ctx, mock.Anything).Return(nil, gatewayErr)
		walletWithdrawer.On("ReverseWithdrawal", ctx, uint(101), "Gateway error: "+gatewayErr.Error()).Return(dbErr)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, gatewayErr)
	})

	t.Run("Success - Payment Repo Create Failed (Should still return success response)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		walletWithdrawer.On("Withdraw", ctx, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", ctx, mock.Anything).Return(dummyPayoutRes, nil)
		paymentRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, res)
	})
}

func TestWithdrawalUsecase_HandlePayoutWebhook(t *testing.T) {
	ctx := context.Background()
	payload := []byte(`{"id":"disb-12345","status":"SUCCEEDED"}`)
	headers := http.Header{"X-Callback-Token": []string{"valid-token"}}
	txID := uint(101)

	validPendingRecord := &entity.PaymentTransaction{
		ID:                  50,
		Provider:            "XENDIT",
		ProviderRefID:       "disb-12345",
		Status:              entity.PaymentTransactionStatusPending,
		WalletTransactionID: &txID,
	}

	t.Run("1. Signature Invalid", func(t *testing.T) {
		usecase, _, paymentDisburser, _ := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(errors.New("invalid signature"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.ErrorIs(t, err, apperror.ErrInvalidWebhookSignature)
	})

	t.Run("2. Parse Payload Gagal", func(t *testing.T) {
		usecase, _, paymentDisburser, _ := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(nil, errors.New("json unmarshal error"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse payout webhook payload")
	})

	t.Run("3. Status PENDING (Skip/No-Op)", func(t *testing.T) {
		usecase, _, paymentDisburser, _ := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "PENDING",
		}, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.NoError(t, err)
	})

	t.Run("4. Record Not Found", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-unknown",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-unknown").Return(nil, apperror.ErrRecordNotFound)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.ErrorIs(t, err, apperror.ErrRecordNotFound)
	})

	t.Run("5. Record Already Resolved", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		resolvedRecord := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusSettled,
			WalletTransactionID: &txID,
		}

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(resolvedRecord, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.NoError(t, err)
	})

	t.Run("6. WalletTransactionID Nil", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		recordWithoutWalletTx := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusPending,
			WalletTransactionID: nil, // NIL!
		}

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(recordWithoutWalletTx, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has no linked wallet transaction id")
	})

	t.Run("7. SUCCEEDED Sukses Penuh", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("FinalizeWithdrawal", ctx, txID).Return(nil)
		paymentRepo.On("UpdateStatus", ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusSettled, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.NoError(t, err)
		walletWithdrawer.AssertExpectations(t)
		paymentRepo.AssertExpectations(t)
	})

	t.Run("8. FinalizeWithdrawal Gagal", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("FinalizeWithdrawal", ctx, txID).Return(errors.New("db error finalize"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "finalize withdrawal")
	})

	t.Run("9. Jalur Success, Tapi UpdateStatus Gagal", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("FinalizeWithdrawal", ctx, txID).Return(nil)
		paymentRepo.On("UpdateStatus", ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusSettled, &txID, &payloadStr).Return(errors.New("db error update"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update payment transaction status")
	})

	t.Run("9. FAILED Sukses Penuh", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("ReverseWithdrawal", ctx, txID, "xendit payout failed").Return(nil)
		paymentRepo.On("UpdateStatus", ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.NoError(t, err)
		walletWithdrawer.AssertExpectations(t)
		paymentRepo.AssertExpectations(t)
	})

	t.Run("10. ReverseWithdrawal Gagal", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("ReverseWithdrawal", ctx, txID, "xendit payout failed").Return(errors.New("db failure"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reverse withdrawal")
	})

	t.Run("11. ReverseWithdrawal Balikin ErrTransactionAlreadyReversed (Dianggap No-Op)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		// ReverseWithdrawal mengembalikan ErrTransactionAlreadyReversed
		walletWithdrawer.On("ReverseWithdrawal", ctx, txID, "xendit payout failed").Return(apperror.ErrTransactionAlreadyReversed)
		// Tetap harus lanjut update status payment repo
		paymentRepo.On("UpdateStatus", ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.NoError(t, err) // Harus return nil (sukses)
		walletWithdrawer.AssertExpectations(t)
		paymentRepo.AssertExpectations(t)
	})

	t.Run("12. Jalur FAILED, Namun Update Status Gagal", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.On("VerifyWebhookSignature", headers).Return(nil)
		paymentDisburser.On("ParsePayoutWebhook", payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)
		paymentRepo.On("FindByProviderRefID", ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.On("ReverseWithdrawal", ctx, txID, "xendit payout failed").Return(nil)
		paymentRepo.On("UpdateStatus", ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(errors.New("db failure"))

		err := usecase.HandlePayoutWebhook(ctx, payload, headers)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update payment transaction status")
	})
}
