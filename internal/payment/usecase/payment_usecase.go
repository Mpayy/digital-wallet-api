package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	"github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	walletdto "github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type WalletTopUpper interface {
	TopUp(ctx context.Context, userID uint, req walletdto.TopUpRequest, idemKey string) (*walletdto.TopUpResponse, error)
}

type PaymentUsecase interface {
	CreateTopUpCheckout(ctx context.Context, userID uint, amount int64) (*dto.CheckoutResponse, error)
	HandleWebhook(ctx context.Context, payload []byte) error
}

type paymentUsecaseImpl struct {
	paymentRepo    repository.PaymentRepository
	gateway        gateway.PaymentCollector
	walletTopUpper WalletTopUpper
	log            *logrus.Logger
}

func NewPaymentUsecase(
	paymentRepo repository.PaymentRepository,
	gateway gateway.PaymentCollector,
	walletTopUpper WalletTopUpper,
	log *logrus.Logger,
) PaymentUsecase {
	return &paymentUsecaseImpl{
		paymentRepo:    paymentRepo,
		gateway:        gateway,
		walletTopUpper: walletTopUpper,
		log:            log,
	}
}

func (p *paymentUsecaseImpl) CreateTopUpCheckout(ctx context.Context, userID uint, amount int64) (*dto.CheckoutResponse, error) {
	logger := p.log.WithFields(logrus.Fields{
		"user_id": userID,
		"amount":  amount,
	})
	logger.Debug("attempting to create top up checkout")

	orderID := uuid.NewString()
	gatewayReq := gateway.ChargeRequest{
		OrderID: orderID,
		Amount:  amount,
	}
	result, err := p.gateway.CreateCharge(ctx, gatewayReq)
	if err != nil {
		return nil, fmt.Errorf("create charge: %w", err)
	}

	record := &entity.PaymentTransaction{
		Provider: "MIDTRANS", ProviderRefID: orderID, UserID: userID,
		Amount: amount, Status: entity.PaymentTransactionStatusPending,
	}

	err = p.paymentRepo.Create(ctx, record)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey){
			return nil, apperror.ErrDuplicatePaymentTransaction
		}
		return nil, fmt.Errorf("save payment transaction: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"order_id": orderID,
	}).Info("checkout created")
	return &dto.CheckoutResponse{
		RedirectURL: result.RedirectURL,
	}, nil
}

func (p *paymentUsecaseImpl) HandleWebhook(ctx context.Context, payload []byte) error {
	err := p.gateway.VerifyWebhookSignature(payload)
	if err != nil {
		p.log.WithError(err).Warn("webhook signature verification failed")
		return apperror.ErrInvalidWebhookSignature
	}

	event, err := p.gateway.ParseWebhookPayload(payload)
	if err != nil {
		return fmt.Errorf("parse webhook payload: %w", err)
	}

	logger := p.log.WithFields(logrus.Fields{
		"provider_ref_id": event.ProviderRefID,
		"status":          event.Status,
	}) // TANPA payload mentah — sama prinsipnya kayak yang kita benerin di IdempotencyService.Claim dulu
	logger.Debug("processing webhook event")

	record, err := p.paymentRepo.FindByProviderRefID(ctx, event.ProviderRefID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			if strings.HasPrefix(event.ProviderRefID, entity.MidtransTestNotifPrefix1) || strings.HasPrefix(event.ProviderRefID, entity.MidtransTestNotifPrefix2) {
				logger.Debug("ignoring midtrans test notification")
				return nil
			}
			logger.Warn("webhook references unknown payment transaction")
			return apperror.ErrInvalidToken
		}
		return fmt.Errorf("find payment transaction: %w", err)
	}

	if record.Status != entity.PaymentTransactionStatusPending {
		logger.Debug("webhook for already-resolved payment transaction, ignoring")
		return nil // webhook duplikat buat record yang udah final — aman diabaikan
	}

	if event.Status != string(entity.PaymentTransactionStatusSettled) {
		payloadStr := string(payload)
		if err := p.paymentRepo.UpdateStatus(ctx, event.ProviderRefID, entity.PaymentTransactionStatus(event.Status), nil, &payloadStr); err != nil {
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
	err = p.paymentRepo.UpdateStatus(ctx, event.ProviderRefID, entity.PaymentTransactionStatusSettled, &topupResp.TransactionID, &payloadStr)
	if err != nil {
		return fmt.Errorf("update payment transaction status: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"user_id": record.UserID, "amount": record.Amount, "wallet_transaction_id": topupResp.TransactionID,
	}).Info("topup completed via webhook")
	return nil
}
