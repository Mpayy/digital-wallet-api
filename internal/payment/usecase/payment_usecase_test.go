package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

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

func newTestLoggerPayment() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func setupPaymentUsecase(t *testing.T) (usecase.PaymentUsecase, *mocks.MockPaymentCollector, *mocks.MockPaymentRepository, *mocks.MockWalletTopUpper, *mocks.MockIdempotencyClaimer) {
	paymentRepo := mocks.NewMockPaymentRepository(t)
	walletTopUpper := mocks.NewMockWalletTopUpper(t)
	idempotencyClaimer := mocks.NewMockIdempotencyClaimer(t)
	paymentCollector := mocks.NewMockPaymentCollector(t)
	log := newTestLoggerPayment()

	usecase := usecase.NewPaymentUsecase(paymentRepo, paymentCollector, walletTopUpper, idempotencyClaimer, log)
	t.Cleanup(func() {
		paymentRepo.AssertExpectations(t)
		paymentCollector.AssertExpectations(t)
		walletTopUpper.AssertExpectations(t)
		idempotencyClaimer.AssertExpectations(t)
	})

	return usecase, paymentCollector, paymentRepo, walletTopUpper, idempotencyClaimer
}

func TestPaymentUsecase_CreateTopUpCheckout(t *testing.T) {
	ctx := context.Background()
	userID := uint(1)
	amount := int64(10000)
	idemKey := "test-idem-key-123"
	dbErr := errors.New("unexpected error")

	t.Run("success_fresh_checkout", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, idempotencyClaimer := setupPaymentUsecase(t)

		// 1. Claim Idempotency OK
		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(true, "", nil)

		// 2. Charge ke Midtrans Gateway OK
		paymentCollector.EXPECT().
			CreateCharge(ctx, mock.MatchedBy(func(req gateway.ChargeRequest) bool {
				return req.Amount == amount && req.OrderID != ""
			})).
			Return(&gateway.ChargeResult{RedirectURL: "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx"}, nil)

		// 3. Simpan Transaksi Pending ke Database OK
		paymentRepo.EXPECT().
			Create(ctx, mock.MatchedBy(func(tx *entity.PaymentTransaction) bool {
				return tx.UserID == userID &&
					tx.Amount == amount &&
					tx.Provider == "MIDTRANS" &&
					tx.Status == entity.PaymentTransactionStatusPending
			})).
			Return(nil)

		// 4. Complete Idempotency OK
		idempotencyClaimer.EXPECT().
			Complete(ctx, idemKey, &dto.CheckoutResponse{
				RedirectURL: "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx",
			}).
			Return(nil)

		response, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx", response.RedirectURL)
	})

	t.Run("success_idempotent_replay", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, idempotencyClaimer := setupPaymentUsecase(t)

		cachedJSON := `{"redirect_url":"https://app.sandbox.midtrans.com/snap/v4/redirection/cached-xxx"}`

		// Claim mengembalikan claimed = false dengan data cache
		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(false, cachedJSON, nil)

		// Gateway & Repo TIDAK dipanggil sama sekali
		response, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "https://app.sandbox.midtrans.com/snap/v4/redirection/cached-xxx", response.RedirectURL)

		// Pastikan repo dan gateway tidak dipanggil
		paymentCollector.AssertNotCalled(t, "CreateCharge")
		paymentRepo.AssertNotCalled(t, "Create")
	})

	t.Run("success_checkout_even_if_complete_fails", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, idempotencyClaimer := setupPaymentUsecase(t)

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(true, "", nil)

		paymentCollector.EXPECT().
			CreateCharge(ctx, mock.MatchedBy(func(req gateway.ChargeRequest) bool {
				return req.Amount == amount && req.OrderID != ""
			})).
			Return(&gateway.ChargeResult{RedirectURL: "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx"}, nil)

		paymentRepo.EXPECT().
			Create(ctx, mock.Anything).
			Return(nil)

		// Simulator: Redis / Lock storage error saat pemanggilan Complete
		idempotencyClaimer.EXPECT().
			Complete(ctx, idemKey, mock.Anything).
			Return(dbErr)

		response, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		// Kegagalan Complete hanya di-log dan TIDAK menggagalkan respon ke client
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx", response.RedirectURL)
	})

	t.Run("failure_idempotency_claim_error", func(t *testing.T) {
		paymentUsecase, _, _, _, idempotencyClaimer := setupPaymentUsecase(t)

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(false, "", dbErr)

		result, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failure_idempotent_corrupted_cache", func(t *testing.T) {
		paymentUsecase, _, _, _, idempotencyClaimer := setupPaymentUsecase(t)

		corruptedJSON := `{invalid-json-content`

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(false, corruptedJSON, nil)

		result, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unmarshal cached checkout response")
	})

	t.Run("failure_gateway_create_charge_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, _, _, idempotencyClaimer := setupPaymentUsecase(t)

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(true, "", nil)

		paymentCollector.EXPECT().
			CreateCharge(ctx, mock.Anything).
			Return(nil, errors.New("midtrans API 500 internal error"))

		// Wajib memanggil MarkFailed agar idempotency key dirilis kembali
		idempotencyClaimer.EXPECT().
			MarkFailed(ctx, idemKey).
			Return(nil)

		res, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "create charge")
	})

	t.Run("failure_duplicate_payment_repo_create_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, idempotencyClaimer := setupPaymentUsecase(t)

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(true, "", nil)

		paymentCollector.EXPECT().
			CreateCharge(ctx, mock.Anything).
			Return(&gateway.ChargeResult{RedirectURL: "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx"}, nil)

		paymentRepo.EXPECT().
			Create(ctx, mock.Anything).
			Return(apperror.ErrDuplicatePayment)

		// Wajib memanggil MarkFailed ketika insert DB gagal
		idempotencyClaimer.EXPECT().
			MarkFailed(ctx, idemKey).
			Return(nil)

		response, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorIs(t, err, apperror.ErrDuplicatePayment)
	})

	t.Run("failure_payment_repo_create_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, idempotencyClaimer := setupPaymentUsecase(t)

		idempotencyClaimer.EXPECT().
			Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", dto.CheckoutRequest{Amount: amount}).
			Return(true, "", nil)

		paymentCollector.EXPECT().
			CreateCharge(ctx, mock.Anything).
			Return(&gateway.ChargeResult{RedirectURL: "https://app.sandbox.midtrans.com/snap/v4/redirection/xxx"}, nil)

		paymentRepo.EXPECT().
			Create(ctx, mock.Anything).
			Return(dbErr)

		// Wajib memanggil MarkFailed ketika insert DB gagal
		idempotencyClaimer.EXPECT().
			MarkFailed(ctx, idemKey).
			Return(nil)

		result, err := paymentUsecase.CreateTopUpCheckout(ctx, userID, amount, idemKey)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, dbErr)
	})
}

func TestPaymentUsecase_HandleWebhook(t *testing.T) {
	ctx := context.Background()
	payload := []byte(`{"order_id":"ORDER-123","transaction_status":"settlement"}`)
	provider := "MIDTRANS"
	providerRefID := "ORDER-123"
	userID := uint(1)
	amount := int64(50000)
	dbErr := errors.New("unexpected error")

	t.Run("success_topup_completed", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		// 1. Verify Signature & Parse Payload OK
		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		// 2. Find Pending Transaction in DB
		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			UserID:        userID,
			Amount:        amount,
			Status:        entity.PaymentTransactionStatusPending,
		}, nil)

		// 3. Call Wallet TopUp with Idempotency Key
		expectedIdemKey := fmt.Sprintf("midtrans-topup:%s", providerRefID)
		expectedWalletTxID := uint(999)
		walletTopUpper.EXPECT().TopUp(ctx, userID, walletdto.TopUpRequest{Amount: amount}, expectedIdemKey).
			Return(&walletdto.TopUpResponse{TransactionID: expectedWalletTxID}, nil)

		// 4. Update Status to SETTLED in DB
		payloadStr := string(payload)
		paymentRepo.EXPECT().UpdateStatus(
			ctx,
			provider,
			providerRefID,
			entity.PaymentTransactionStatusSettled,
			&expectedWalletTxID,
			&payloadStr,
		).Return(nil)

		err := paymentUsecase.HandleWebhook(ctx, payload)
		assert.NoError(t, err)
	})

	t.Run("success_non_settled_status_updated", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		// 1. Verify Signature & Parse Payload OK
		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusExpired), // Status non-settled
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			UserID:        userID,
			Amount:        amount,
			Status:        entity.PaymentTransactionStatusPending,
		}, nil)

		// Update status DB ke "expire" tanpa memanggil Wallet.TopUp!
		payloadStr := string(payload)
		paymentRepo.EXPECT().UpdateStatus(
			ctx,
			provider,
			providerRefID,
			entity.PaymentTransactionStatusExpired,
			mock.MatchedBy(func(id *uint) bool { return id == nil }),
			&payloadStr,
		).Return(nil)

		err := paymentUsecase.HandleWebhook(ctx, payload)
		assert.NoError(t, err)

		walletTopUpper.AssertNotCalled(t, "TopUp")
	})

	t.Run("success_already_resolved_transaction_ignored", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		// Status transaksi di DB sudah SETTLED (bukan PENDING lagi)
		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			Status:        entity.PaymentTransactionStatusSettled,
		}, nil)

		// Langsung diabaikan (nil error), tidak ada panggilan TopUp / UpdateStatus
		err := paymentUsecase.HandleWebhook(ctx, payload)
		assert.NoError(t, err)
		walletTopUpper.AssertNotCalled(t, "TopUp")
		paymentRepo.AssertNotCalled(t, "UpdateStatus")
	})

	t.Run("success_ignore_midtrans_test_notif", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		testRefID := gateway.MidtransTestNotifPrefix1 + "-12345"

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: testRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		// Record tidak ada di DB
		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, testRefID).
			Return(nil, apperror.ErrRecordNotFound)

		// Otomatis di-bypass dan return nil karena terdeteksi sebagai notifikasi testing Midtrans
		err := paymentUsecase.HandleWebhook(ctx, payload)
		assert.NoError(t, err)
		walletTopUpper.AssertNotCalled(t, "TopUp")
	})

	t.Run("failure_invalid_signature", func(t *testing.T) {
		paymentUsecase, paymentCollector, _, _, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).
			Return(nil, apperror.ErrInvalidWebhookSignature)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.Error(t, err)
		assert.ErrorIs(t, err, apperror.ErrInvalidWebhookSignature)
	})

	t.Run("failure_parse_payload_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).
			Return(nil, errors.New("unexpected EOF JSON"))

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse webhook payload")
		paymentRepo.AssertNotCalled(t, "FindByProviderRefID")
	})

	t.Run("failure_unknown_payment_transaction", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: "UNKNOWN-ORDER-999",
			Status:        "settlement",
		}, nil)

		// Transaction tidak ada & BUKAN notifikasi test
		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, "UNKNOWN-ORDER-999").
			Return(nil, apperror.ErrRecordNotFound)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.Error(t, err)
		assert.ErrorIs(t, err, apperror.ErrRecordNotFound)
	})

	t.Run("failure_db_find_transaction_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).
			Return(nil, dbErr)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failure_non_settled_status_update_db_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, _, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusFailed),
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			Status:        entity.PaymentTransactionStatusPending,
		}, nil)

		payloadStr := string(payload)
		paymentRepo.EXPECT().UpdateStatus(
			ctx,
			provider,
			providerRefID,
			entity.PaymentTransactionStatusFailed,
			mock.MatchedBy(func(id *uint) bool { return id == nil }),
			&payloadStr,
		).Return(dbErr)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failure_topup_wallet_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			UserID:        userID,
			Amount:        amount,
			Status:        entity.PaymentTransactionStatusPending,
		}, nil)

		// Simulator: Module wallet error saat TopUp
		expectedIdemKey := fmt.Sprintf("midtrans-topup:%s", providerRefID)
		walletTopUpper.EXPECT().TopUp(ctx, userID, walletdto.TopUpRequest{Amount: amount}, expectedIdemKey).
			Return(nil, dbErr)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("failure_update_settled_status_db_error", func(t *testing.T) {
		paymentUsecase, paymentCollector, paymentRepo, walletTopUpper, _ := setupPaymentUsecase(t)

		paymentCollector.EXPECT().VerifyAndParseWebhook(payload).Return(&gateway.WebhookEvent{
			ProviderRefID: providerRefID,
			Status:        string(entity.PaymentTransactionStatusSettled),
		}, nil)

		paymentRepo.EXPECT().FindByProviderRefID(ctx, provider, providerRefID).Return(&entity.PaymentTransaction{
			ProviderRefID: providerRefID,
			UserID:        userID,
			Amount:        amount,
			Status:        entity.PaymentTransactionStatusPending,
		}, nil)

		walletTxID := uint(999)
		walletTopUpper.EXPECT().TopUp(ctx, userID, mock.Anything, mock.Anything).
			Return(&walletdto.TopUpResponse{TransactionID: walletTxID}, nil)

		payloadStr := string(payload)
		paymentRepo.EXPECT().UpdateStatus(ctx, provider, providerRefID, entity.PaymentTransactionStatusSettled, &walletTxID, &payloadStr).
			Return(dbErr)

		err := paymentUsecase.HandleWebhook(ctx, payload)

		assert.ErrorIs(t, err, dbErr)
	})
}
