package usecase_test

import (
	"context"
	"errors"
	"fmt"
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
	pkgmocks "github.com/Mpayy/digital-wallet-api/internal/pkg/mocks"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
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

func setupWithdrawalUsecase(t *testing.T) (usecase.WithdrawalUsecase, *mocks.MockWalletWithdrawer, *mocks.MockPaymentDisburser, *mocks.MockPaymentRepository, *pkgmocks.MockPublisher) {
	paymentRepo := mocks.NewMockPaymentRepository(t)
	walletWithdrawer := mocks.NewMockWalletWithdrawer(t)
	paymentDisburser := mocks.NewMockPaymentDisburser(t)
	publisher := pkgmocks.NewMockPublisher(t)

	log := newTestLoggerWithdrawal()

	usecase := usecase.NewWithdrawalUsecase(paymentRepo, paymentDisburser, walletWithdrawer, publisher, log)
	t.Cleanup(func() {
		paymentRepo.AssertExpectations(t)
		paymentDisburser.AssertExpectations(t)
		walletWithdrawer.AssertExpectations(t)
		publisher.AssertExpectations(t)
	})

	return usecase, walletWithdrawer, paymentDisburser, paymentRepo, publisher
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

	now := time.Now()

	dummyWithdrawRes := &walletdto.WithdrawResponse{
		TransactionID: 101,
		WalletID:      1,
		Amount:        100000,
		BalanceBefore: 500000,
		BalanceAfter:  400000,
		CreatedAt:     now,
	}

	dummyPayoutRes := &gateway.PayoutResult{
		ProviderRefID: "disb-12345",
		Status:        "PENDING",
	}

	t.Run("success_create_withdrawal", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)
		expectedOrderID := fmt.Sprintf("WITHDRAWAL-%d-%d", dummyWithdrawRes.TransactionID, now.Unix())
		walletWithdrawer.On("Withdraw", mock.Anything, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", mock.Anything, gateway.PayoutRequest{
			ReferenceID:       expectedOrderID,
			ChannelCode:       req.ChannelCode,
			AccountNumber:     req.AccountNumber,
			AccountHolderName: req.AccountHolderName,
			Amount:            req.Amount,
		}).Return(dummyPayoutRes, nil)

		paymentRepo.On("Create", mock.Anything, mock.MatchedBy(func(record *entity.PaymentTransaction) bool {
			return record.Provider == "XENDIT" && record.ProviderRefID == "disb-12345" && *record.WalletTransactionID == uint(101)
		})).Return(nil)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, uint(101), res.TransactionID)
		assert.Equal(t, "disb-12345", res.ReferenceID)
	})

	t.Run("error_wallet_withdraw", func(t *testing.T) {
		usecase, walletWithdrawer, _, _, _ := setupWithdrawalUsecase(t)

		withDrawErr := errors.New("Withdraw Error")
		walletWithdrawer.On("Withdraw", mock.Anything, userID, req.Amount, idemKey).Return(nil, withDrawErr)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, withDrawErr)
	})

	t.Run("error_gateway_error_reversal_succeeded", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, _, _ := setupWithdrawalUsecase(t)

		gatewayErr := errors.New("xendit connection timeout")
		walletWithdrawer.On("Withdraw", mock.Anything, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", mock.Anything, mock.Anything).Return(nil, gatewayErr)
		walletWithdrawer.On("ReverseWithdrawal", mock.Anything, uint(101), "Gateway error: xendit connection timeout").Return(nil)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, gatewayErr)
	})

	t.Run("error_gateway_error_reversal_failed (critical)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, _, _ := setupWithdrawalUsecase(t)

		gatewayErr := errors.New("xendit connection timeout")
		dbErr := errors.New("db lock timeout")

		walletWithdrawer.On("Withdraw", mock.Anything, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", mock.Anything, mock.Anything).Return(nil, gatewayErr)
		walletWithdrawer.On("ReverseWithdrawal", mock.Anything, uint(101), "Gateway error: "+gatewayErr.Error()).Return(dbErr)

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, gatewayErr)
	})

	t.Run("error_payment_repo_create_failed (should still return success response)", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		walletWithdrawer.On("Withdraw", mock.Anything, userID, req.Amount, idemKey).Return(dummyWithdrawRes, nil)
		paymentDisburser.On("CreatePayout", mock.Anything, mock.Anything).Return(dummyPayoutRes, nil)
		paymentRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db error"))

		res, err := usecase.CreateWithdrawal(ctx, userID, req, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, res)
	})
}

func TestWithdrawalUsecase_HandlePayoutWebhook(t *testing.T) {
	ctx := context.Background()
	payload := []byte(`{"id":"disb-12345","status":"SUCCEEDED"}`)
	txID := uint(101)

	validPendingRecord := &entity.PaymentTransaction{
		ID:                  50,
		Provider:            "XENDIT",
		ProviderRefID:       "disb-12345",
		Status:              entity.PaymentTransactionStatusPending,
		WalletTransactionID: &txID,
	}

	t.Run("error_parse_payout_webhook_failed", func(t *testing.T) {
		usecase, _, paymentDisburser, _, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(nil, errors.New("json unmarshal error"))

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse payout webhook payload")
	})

	t.Run("status_pending_ignore_no_op", func(t *testing.T) {
		usecase, _, paymentDisburser, _, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "PENDING",
		}, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("error_record_not_found", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-unknown",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-unknown").Return(nil, apperror.ErrRecordNotFound)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.ErrorIs(t, err, apperror.ErrRecordNotFound)
	})

	t.Run("record_already_resolved", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		resolvedRecord := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusSettled,
			WalletTransactionID: &txID,
		}

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(resolvedRecord, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("wallet_transaction_id_nil", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		recordWithoutWalletTx := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusPending,
			WalletTransactionID: nil, // NIL!
		}

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(recordWithoutWalletTx, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has no linked wallet transaction id")
	})

	t.Run("succeeded_full_success", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().FinalizeWithdrawal(ctx, txID).Return(nil)

		paymentRepo.EXPECT().UpdateStatus(ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusSettled, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("succeeded_finalize_failed", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().FinalizeWithdrawal(ctx, txID).Return(errors.New("db error finalize"))

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "finalize withdrawal")
	})

	t.Run("succeeded_update_status_failed", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "SUCCEEDED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)
		walletWithdrawer.EXPECT().FinalizeWithdrawal(ctx, txID).Return(nil)
		paymentRepo.EXPECT().UpdateStatus(ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusSettled, &txID, &payloadStr).Return(errors.New("db error update"))

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update payment transaction status settled")
	})

	t.Run("failed_webhook_full_success", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(mock.Anything, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().ReverseWithdrawal(mock.Anything, txID, "xendit payout failed").Return(nil)

		paymentRepo.EXPECT().UpdateStatus(mock.Anything, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("failed_webhook_reverse_withdrawal_failed_transaction_already_reversed", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().ReverseWithdrawal(ctx, txID, "xendit payout failed").Return(apperror.ErrTransactionAlreadyReversed)

		paymentRepo.EXPECT().UpdateStatus(ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("failed_webhook_reverse_withdrawal_failed", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().ReverseWithdrawal(ctx, txID, "xendit payout failed").Return(errors.New("db error reverse"))

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Contains(t, err.Error(), "reverse withdrawal")
	})

	t.Run("failed_webhook_update_failed", func(t *testing.T) {
		usecase, walletWithdrawer, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		payloadStr := string(payload)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(validPendingRecord, nil)

		walletWithdrawer.EXPECT().ReverseWithdrawal(mock.Anything, txID, "xendit payout failed").Return(nil)

		paymentRepo.EXPECT().UpdateStatus(ctx, "XENDIT", "disb-12345", entity.PaymentTransactionStatusFailed, &txID, &payloadStr).Return(errors.New("db error update"))

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update payment transaction status failed")
	})

	t.Run("error_final_record_already_settled", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		recordingRecord := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusSettled,
			WalletTransactionID: &txID,
		}

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(recordingRecord, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("error_final_record_already_failed", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		recordingRecord := &entity.PaymentTransaction{
			ID:                  50,
			ProviderRefID:       "disb-12345",
			Status:              entity.PaymentTransactionStatusFailed,
			WalletTransactionID: &txID,
		}

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(recordingRecord, nil)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.NoError(t, err)
	})

	t.Run("error_payment_repo_find_failed", func(t *testing.T) {
		usecase, _, paymentDisburser, paymentRepo, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().ParsePayoutWebhook(payload).Return(&gateway.PayoutWebhookEvent{
			ProviderRefID: "disb-12345",
			Status:        "FAILED",
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, "XENDIT", "disb-12345").Return(nil, apperror.ErrRecordNotFound)

		err := usecase.HandlePayoutWebhook(ctx, payload)

		assert.Error(t, err)
		assert.ErrorIs(t, err, apperror.ErrRecordNotFound)
	})
}

func TestWithdrawalUsecase_ReceivePayoutWebhook(t *testing.T) {
	ctx := context.Background()
	headers := http.Header{"X-Callback-Token": []string{"valid-token"}}
	payload := []byte(`{"id":"disb-12345","status":"SUCCEEDED"}`)

	t.Run("success", func(t *testing.T) {
		usecase, _, paymentDisburser, _, publisher := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().VerifyWebhookSignature(headers).Return(nil)
		publisher.EXPECT().Publish(ctx, queue.XenditWebhookQueue, payload).Return(nil)

		err := usecase.ReceivePayoutWebhook(ctx, headers, payload)

		assert.NoError(t, err)
	})

	t.Run("failed_invalid_webhook_signature", func(t *testing.T) {
		usecase, _, paymentDisburser, _, _ := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().VerifyWebhookSignature(headers).Return(errors.New("invalid callback token"))

		err := usecase.ReceivePayoutWebhook(ctx, headers, payload)

		assert.ErrorIs(t, err, apperror.ErrInvalidWebhookSignature)
	})

	t.Run("failed_publish_message", func(t *testing.T) {
		usecase, _, paymentDisburser, _, publisher := setupWithdrawalUsecase(t)

		paymentDisburser.EXPECT().VerifyWebhookSignature(headers).Return(nil)
		publisher.EXPECT().Publish(ctx, queue.XenditWebhookQueue, payload).Return(errors.New("rabbitmq connection down"))

		err := usecase.ReceivePayoutWebhook(ctx, headers, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "publish webhook message")
	})

}
