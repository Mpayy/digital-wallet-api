package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/payment/gateway"
	"github.com/Mpayy/digital-wallet-api/internal/payment/repository"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
	walletdto "github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/sirupsen/logrus"
)

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_wallet_withdrawer.go
type WalletWithdrawer interface {
	Withdraw(ctx context.Context, userID uint, amount int64, idemKey string) (*walletdto.WithdrawResponse, error)
	ReverseWithdrawal(ctx context.Context, transactionID uint, reason string) error
	FinalizeWithdrawal(ctx context.Context, transactionID uint) error
}

type WithdrawalUsecase interface {
	CreateWithdrawal(ctx context.Context, userID uint, req dto.WithdrawalRequest, idemKey string) (*dto.WithdrawalResponse, error)
	HandlePayoutWebhook(ctx context.Context, payload []byte) error
	ReceivePayoutWebhook(ctx context.Context, headers http.Header, payload []byte) error
}

type withdrawalUsecaseImpl struct {
	paymentRepo      repository.PaymentRepository
	gateway          gateway.PaymentDisburser
	walletWithdrawal WalletWithdrawer
	publisher        queue.Publisher
	log              *logrus.Logger
}

func NewWithdrawalUsecase(paymentRepo repository.PaymentRepository, gateway gateway.PaymentDisburser, walletWithdrawal WalletWithdrawer, publisher queue.Publisher, log *logrus.Logger) WithdrawalUsecase {
	return &withdrawalUsecaseImpl{
		paymentRepo:      paymentRepo,
		gateway:          gateway,
		walletWithdrawal: walletWithdrawal,
		publisher:        publisher,
		log:              log,
	}
}

func (w *withdrawalUsecaseImpl) CreateWithdrawal(ctx context.Context, userID uint, req dto.WithdrawalRequest, idemKey string) (*dto.WithdrawalResponse, error) {
	logger := w.log.WithFields(logrus.Fields{
		"user_id": userID,
		"amount":  req.Amount,
	})
	logger.Debug("attempting to create withdrawal")

	withdrawRes, err := w.walletWithdrawal.Withdraw(ctx, userID, req.Amount, idemKey)
	if err != nil {
		return nil, err
	}

	orderID := fmt.Sprintf("WITHDRAWAL-%d-%d", withdrawRes.TransactionID, withdrawRes.CreatedAt.Unix())
	payoutReq := gateway.PayoutRequest{
		ReferenceID:       orderID,
		ChannelCode:       req.ChannelCode,
		AccountNumber:     req.AccountNumber,
		AccountHolderName: req.AccountHolderName,
		Amount:            req.Amount,
	}

	result, err := w.gateway.CreatePayout(ctx, payoutReq)
	if err != nil {
		reverseErr := w.walletWithdrawal.ReverseWithdrawal(ctx, withdrawRes.TransactionID, "Gateway error: "+err.Error())
		if reverseErr != nil {
			logger.WithFields(logrus.Fields{
				"transaction_id": withdrawRes.TransactionID,
				"reverse_error":  reverseErr,
				"gateway_error":  err,
			}).Error("CRITICAL: withdrawal debited, gateway payout failed, AND reversal also failed — funds stuck, manual reconciliation required")
		}
		return nil, fmt.Errorf("gateway create payout: %w", err)
	}

	record := &entity.PaymentTransaction{
		Provider:            "XENDIT",
		ProviderRefID:       result.ProviderRefID,
		UserID:              userID,
		Type:                withdrawRes.Type,
		Amount:              req.Amount,
		Status:              entity.PaymentTransactionStatusPending,
		WalletTransactionID: &withdrawRes.TransactionID,
	}

	err = w.paymentRepo.Create(ctx, record)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			logger.Debug("payment transaction record already exists for this reference — idempotent replay")
		} else {
			logger.WithFields(logrus.Fields{
				"provider_ref_id":       result.ProviderRefID,
				"wallet_transaction_id": withdrawRes.TransactionID,
				"amount":                req.Amount,
				"error":                 err,
			}).Error("CRITICAL: xendit payout created but payment_transaction record failed to save — webhook for this payout will have nowhere to land, requires manual reconciliation")
		}
	}

	return &dto.WithdrawalResponse{
		TransactionID:     withdrawRes.TransactionID,
		WalletID:          withdrawRes.WalletID,
		Amount:            withdrawRes.Amount,
		BalanceBefore:     withdrawRes.BalanceBefore,
		BalanceAfter:      withdrawRes.BalanceAfter,
		ChannelCode:       req.ChannelCode,
		AccountNumber:     req.AccountNumber,
		AccountHolderName: req.AccountHolderName,
		ReferenceID:       result.ProviderRefID,
		Status:            result.Status,
		CreatedAt:         withdrawRes.CreatedAt,
	}, nil
}

func (w *withdrawalUsecaseImpl) HandlePayoutWebhook(ctx context.Context, payload []byte) error {
	event, err := w.gateway.ParsePayoutWebhook(payload)
	if err != nil {
		return fmt.Errorf("parse payout webhook payload: %w", err)
	}

	logger := w.log.WithFields(logrus.Fields{
		"provider_ref_id": event.ProviderRefID,
		"status":          event.Status,
	})

	if event.Status == "PENDING" {
		logger.Debug("payout webhook is a transitional status, ignoring")
		return nil
	}

	record, err := w.paymentRepo.FindByProviderRefID(ctx, "XENDIT", event.ProviderRefID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			logger.Warn("payout webhook references unknown payment transaction")
			return apperror.ErrRecordNotFound
		}
		return fmt.Errorf("find payment transaction: %w", err)
	}

	if record.Status != entity.PaymentTransactionStatusPending {
		logger.Debug("payout webhook for already-resolved transaction, ignoring")
		return nil
	}

	if record.WalletTransactionID == nil {
		return fmt.Errorf("payment transaction %d has no linked wallet transaction id", record.ID)
	}

	payloadStr := string(payload)

	switch event.Status {
	case "SUCCEEDED":
		err := w.walletWithdrawal.FinalizeWithdrawal(ctx, *record.WalletTransactionID)
		if err != nil {
			return fmt.Errorf("finalize withdrawal: %w", err)
		}
		err = w.paymentRepo.UpdateStatus(ctx, "XENDIT", event.ProviderRefID, entity.PaymentTransactionStatusSettled, record.WalletTransactionID, &payloadStr)
		if err != nil {
			return fmt.Errorf("update payment transaction status settled: %w", err)
		}
		logger.WithFields(logrus.Fields{"wallet_transaction_id": *record.WalletTransactionID}).Info("withdrawal finalized via webhook")

	case "FAILED":
		if err := w.walletWithdrawal.ReverseWithdrawal(ctx, *record.WalletTransactionID, "xendit payout failed"); err != nil {
			if !errors.Is(err, apperror.ErrTransactionAlreadyReversed) {
				return fmt.Errorf("reverse withdrawal: %w", err)
			}
			logger.Debug("withdrawal already reversed, treating as no-op")
		}
		if err := w.paymentRepo.UpdateStatus(ctx, "XENDIT", event.ProviderRefID, entity.PaymentTransactionStatusFailed, record.WalletTransactionID, &payloadStr); err != nil {
			return fmt.Errorf("update payment transaction status failed: %w", err)
		}
		logger.WithFields(logrus.Fields{"wallet_transaction_id": *record.WalletTransactionID}).Info("withdrawal reversed via webhook, funds returned to wallet")
	default:
		return fmt.Errorf("unhandled payout status: %s", event.Status)
	}

	return nil
}

func (w *withdrawalUsecaseImpl) ReceivePayoutWebhook(ctx context.Context, headers http.Header, payload []byte) error {
	if err := w.gateway.VerifyWebhookSignature(headers); err != nil {
		return apperror.ErrInvalidWebhookSignature
	}
	if err := w.publisher.Publish(ctx, queue.XenditWebhookQueue, payload); err != nil {
		return fmt.Errorf("publish webhook message: %w", err)
	}
	return nil
}
