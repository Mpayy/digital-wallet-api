package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TransferUsecase interface {
	Transfer(ctx context.Context, fromUserID uint, request dto.TransferRequest, idemKey string) (*dto.TransferResponse, error)
}

type transferUsecaseImpl struct {
	transferRepo    repository.TransferRepository
	walletRepo      repository.WalletRepository
	idemService     IdempotencyService
	transactionRepo repository.TransactionRepository
	log             *logrus.Logger
}

func NewTransferUsecase(transferRepo repository.TransferRepository, walletRepo repository.WalletRepository, idemService IdempotencyService, transactionRepo repository.TransactionRepository, log *logrus.Logger) TransferUsecase {
	return &transferUsecaseImpl{
		transferRepo:    transferRepo,
		walletRepo:      walletRepo,
		idemService:     idemService,
		transactionRepo: transactionRepo,
		log:             log,
	}
}

func (u *transferUsecaseImpl) Transfer(ctx context.Context, fromUserID uint, request dto.TransferRequest, idemKey string) (*dto.TransferResponse, error) {
	logger := u.log.WithFields(logrus.Fields{
		"from_user_id": fromUserID,
		"to_user_id":   request.ToUserID,
		"amount":       request.Amount,
		"idem_key":     idemKey,
	})
	logger.Debug("attempting transfer")

	if request.Amount <= 0 {
		return nil, apperror.ErrInvalidAmount
	}

	fromWallet, err := u.walletRepo.FindByUserID(ctx, fromUserID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrWalletNotFound
		}
		return nil, fmt.Errorf("find sender wallet: %w", err)
	}

	toWallet, err := u.walletRepo.FindByUserID(ctx, request.ToUserID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrWalletNotFound
		}
		return nil, fmt.Errorf("find recipient wallet: %w", err)
	}

	if fromWallet.ID == toWallet.ID {
		logger.Warn("self transfer not allowed")
		return nil, apperror.ErrSelfTransferNotAllowed
	}

	claimed, cacheBody, err := u.idemService.Claim(ctx, idemKey, fromUserID, "TRANSFER", request)
	if err != nil {
		return nil, err
	}

	if !claimed {
		var cached dto.TransferResponse
		if err := json.Unmarshal([]byte(cacheBody), &cached); err != nil {
			return nil, fmt.Errorf("unmarshal cached transfer response: %w", err)
		}

		logger.Info("transfer request replayed from idempotency cache")
		return &cached, nil
	}

	var result *dto.TransferResponse
	txErr := u.walletRepo.WithTx(ctx, func(tx *gorm.DB) error {
		firstID, secondID := fromWallet.ID, toWallet.ID
		if firstID > secondID {
			firstID, secondID = secondID, firstID
		}

		firstLocked, errLock1 := u.walletRepo.LockByID(tx, firstID)
		if errLock1 != nil {
			if errors.Is(errLock1, apperror.ErrRecordNotFound) {
				return apperror.ErrWalletNotFound
			}

			return fmt.Errorf("lock sender wallet: %w", errLock1)
		}

		secondLocked, errLock2 := u.walletRepo.LockByID(tx, secondID)
		if errLock2 != nil {
			if errors.Is(errLock2, apperror.ErrRecordNotFound) {
				return apperror.ErrWalletNotFound
			}

			return fmt.Errorf("lock recipient wallet: %w", errLock2)
		}

		var sender, recipient *entity.Wallet
		if firstLocked.ID == fromWallet.ID {
			sender, recipient = firstLocked, secondLocked
		} else {
			sender, recipient = secondLocked, firstLocked
		}

		if sender.Balance < request.Amount {
			return apperror.ErrInsufficientBalance
		}

		senderBefore, recipientBefore := sender.Balance, recipient.Balance

		sender.Balance -= request.Amount
		recipient.Balance += request.Amount

		errSaveSender := u.walletRepo.Save(tx, sender)
		if errSaveSender != nil {
			return fmt.Errorf("save sender wallet: %w", errSaveSender)
		}

		errSaveRecipient := u.walletRepo.Save(tx, recipient)
		if errSaveRecipient != nil {
			return fmt.Errorf("save recipient wallet: %w", errSaveRecipient)
		}

		transferTx := &entity.Transfer{
			FromWalletID: fromWallet.ID,
			ToWalletID:   toWallet.ID,
			Amount:       request.Amount,
			Note:         request.Note,
		}

		errCreateTransfer := u.transferRepo.Create(tx, transferTx)
		if errCreateTransfer != nil {
			return fmt.Errorf("create transfer record: %w", errCreateTransfer)
		}

		outTx := &entity.Transaction{
			WalletID:      sender.ID,
			Type:          entity.TxTypeTransferOut,
			Amount:        request.Amount,
			BalanceBefore: senderBefore,
			BalanceAfter:  sender.Balance,
			TransferID:    &transferTx.ID,
			Status:        entity.TxStatusSuccess,
		}

		inTx := &entity.Transaction{
			WalletID:      recipient.ID,
			Type:          entity.TxTypeTransferIn,
			Amount:        request.Amount,
			BalanceBefore: recipientBefore,
			BalanceAfter:  recipient.Balance,
			TransferID:    &transferTx.ID,
			Status:        entity.TxStatusSuccess,
		}

		errCreateOutTx := u.transactionRepo.Create(tx, outTx)
		if errCreateOutTx != nil {
			return fmt.Errorf("create outgoing transaction: %w", errCreateOutTx)
		}
		errCreateInTx := u.transactionRepo.Create(tx, inTx)
		if errCreateInTx != nil {
			return fmt.Errorf("create incoming transaction: %w", errCreateInTx)
		}

		result = &dto.TransferResponse{
			TransferID:         transferTx.ID,
			TransactionID:      outTx.ID,
			Type:               string(entity.TxTypeTransferOut),
			Amount:             request.Amount,
			BalanceBefore:      senderBefore,
			BalanceAfter:       sender.Balance,
			CounterPartyUserID: recipient.UserID,
			Status:             string(entity.TxStatusSuccess),
			CreatedAt:          transferTx.CreatedAt,
		}

		return nil
	})

	if txErr != nil {
		errMarkFailed := u.idemService.MarkFailed(ctx, idemKey)
		if errMarkFailed != nil {
			logger.WithError(errMarkFailed).Error("transfer failed but failed to mark idempotency failed")
		}
		return nil, txErr
	}

	errComplete := u.idemService.Complete(ctx, idemKey, result)
	if errComplete != nil {
		logger.WithError(errComplete).Error("transfer completed but failed to mark idempotency completed")
	}

	logger.Info("transfer completed successfully")
	return result, nil
}
