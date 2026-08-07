package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	"github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
	walletdto "github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_idempotency_claimer.go
type IdempotencyClaimer interface {
	Claim(ctx context.Context, key string, userID uint, endpoint string, payload any) (claimed bool, cachedBody string, err error)
	Complete(ctx context.Context, key string, response any) error
	MarkFailed(ctx context.Context, key string) error
}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_wallet_top_upper.go
type WalletTopUpper interface {
	TopUp(ctx context.Context, userID uint, req walletdto.TopUpRequest, idemKey string) (*walletdto.TopUpResponse, error)
}

type PaymentUsecase interface {
	CreateTopUpCheckout(ctx context.Context, userID uint, amount int64, idemKey string) (*dto.CheckoutResponse, error)
	HandleWebhook(ctx context.Context, payload []byte) error
	ReceiveWebhook(ctx context.Context, payload []byte) error
}

type paymentUsecaseImpl struct {
	paymentRepo        repository.PaymentRepository
	gateway            gateway.PaymentCollector
	walletTopUpper     WalletTopUpper
	idempotencyClaimer IdempotencyClaimer
	log                *logrus.Logger
	publisher          queue.Publisher
}

func NewPaymentUsecase(
	paymentRepo repository.PaymentRepository,
	gateway gateway.PaymentCollector,
	walletTopUpper WalletTopUpper,
	idempotencyClaimer IdempotencyClaimer,
	log *logrus.Logger,
	publisher queue.Publisher,
) PaymentUsecase {
	return &paymentUsecaseImpl{
		paymentRepo:        paymentRepo,
		gateway:            gateway,
		walletTopUpper:     walletTopUpper,
		idempotencyClaimer: idempotencyClaimer,
		log:                log,
		publisher:          publisher,
	}
}

func (p *paymentUsecaseImpl) CreateTopUpCheckout(ctx context.Context, userID uint, amount int64, idemKey string) (*dto.CheckoutResponse, error) {
	logger := p.log.WithFields(logrus.Fields{
		"user_id": userID,
		"amount":  amount,
	})
	logger.Debug("attempting to create top up checkout")

	if amount <= 0 {
		return nil, apperror.ErrInvalidAmount
	}

	payload := dto.CheckoutRequest{
		Amount: amount,
	}

	claimed, cachedBody, err := p.idempotencyClaimer.Claim(ctx, idemKey, userID, "TOPUP_CHECKOUT", payload)
	if err != nil {
		return nil, err
	}

	if !claimed {
		var cached dto.CheckoutResponse
		if err := json.Unmarshal([]byte(cachedBody), &cached); err != nil {
			return nil, fmt.Errorf("unmarshal cached checkout response: %w", err)
		}
		logger.Info("checkout replayed from idempotency cache") // <- INI kuncinya: link Midtrans yang SAMA dibalikin lagi
		return &cached, nil
	}

	orderID := uuid.NewString()
	gatewayReq := gateway.ChargeRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	result, err := p.gateway.CreateCharge(ctx, gatewayReq)
	if err != nil {
		markErr := p.idempotencyClaimer.MarkFailed(ctx, idemKey)
		if markErr != nil {
			logger.WithError(markErr).Error("failed to mark idempotency key as failed")
		}
		return nil, fmt.Errorf("create charge: %w", err)
	}

	record := &entity.PaymentTransaction{
		Provider:      "MIDTRANS",
		ProviderRefID: orderID,
		UserID:        userID,
		Type:          "TOPUP",
		Amount:        amount,
		Status:        entity.PaymentTransactionStatusPending,
	}

	err = p.paymentRepo.Create(ctx, record)
	if err != nil {
		markErr := p.idempotencyClaimer.MarkFailed(ctx, idemKey)
		if markErr != nil {
			logger.WithError(markErr).Error("failed to mark idempotency key as failed")
		}
		return nil, fmt.Errorf("save payment transaction: %w", err)
	}

	checkoutResponse := &dto.CheckoutResponse{
		RedirectURL: result.RedirectURL,
	}

	err = p.idempotencyClaimer.Complete(ctx, idemKey, checkoutResponse)
	if err != nil {
		logger.WithError(err).Error("checkout created but failed to mark idempotency completed")
	}

	logger.WithFields(logrus.Fields{
		"order_id": orderID,
	}).Info("checkout created")
	return checkoutResponse, nil
}

func (p *paymentUsecaseImpl) HandleWebhook(ctx context.Context, payload []byte) error {
	event, err := p.gateway.ParseWebhookPayload(payload)
	if err != nil {
		return fmt.Errorf("parse webhook payload: %w", err)
	}

	logger := p.log.WithFields(logrus.Fields{
		"provider_ref_id": event.ProviderRefID,
		"status":          event.Status,
	}) // TANPA payload mentah — sama prinsipnya kayak yang kita benerin di IdempotencyService.Claim dulu
	logger.Debug("processing webhook event")

	record, err := p.paymentRepo.FindByProviderRefID(ctx, "MIDTRANS", event.ProviderRefID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			if strings.HasPrefix(event.ProviderRefID, gateway.MidtransTestNotifPrefix1) || strings.HasPrefix(event.ProviderRefID, gateway.MidtransTestNotifPrefix2) {
				logger.Debug("ignoring midtrans test notification")
				return nil
			}
			logger.Warn("webhook references unknown payment transaction")
			return apperror.ErrRecordNotFound
		}
		return fmt.Errorf("find payment transaction: %w", err)
	}

	if record.Status != entity.PaymentTransactionStatusPending {
		logger.Debug("webhook for already-resolved payment transaction, ignoring")
		return nil // webhook duplikat buat record yang udah final — aman diabaikan
	}

	if event.Status != string(entity.PaymentTransactionStatusSettled) {
		payloadStr := string(payload)
		if err := p.paymentRepo.UpdateStatus(ctx, "MIDTRANS", event.ProviderRefID, entity.PaymentTransactionStatus(event.Status), nil, &payloadStr); err != nil {
			return fmt.Errorf("update payment transaction status to %s: %w", event.Status, err)
		}
		logger.Info("payment did not settle")
		return nil
	}

	idemkey := fmt.Sprintf("midtrans-topup:%s", event.ProviderRefID)
	topupReq := walletdto.TopUpRequest{
		Amount: record.Amount,
	}

	topupResp, err := p.walletTopUpper.TopUp(ctx, record.UserID, topupReq, idemkey)
	if err != nil {
		return fmt.Errorf("top up wallet: %w", err)
	}

	payloadStr := string(payload)
	err = p.paymentRepo.UpdateStatus(ctx, "MIDTRANS", event.ProviderRefID, entity.PaymentTransactionStatusSettled, &topupResp.TransactionID, &payloadStr)
	if err != nil {
		return fmt.Errorf("update payment transaction status: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"user_id": record.UserID, "amount": record.Amount, "wallet_transaction_id": topupResp.TransactionID,
	}).Info("topup completed via webhook")
	return nil
}

func (p *paymentUsecaseImpl) ReceiveWebhook(ctx context.Context, payload []byte) error {
	if err := p.gateway.VerifyWebhookSignature(payload); err != nil {
		return apperror.ErrInvalidWebhookSignature
	}
	if err := p.publisher.Publish(ctx, queue.MidtransWebhookQueue, payload); err != nil {
		return fmt.Errorf("publish webhook message: %w", err)
	}
	return nil
}
