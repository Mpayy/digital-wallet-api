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

type WalletUsecase interface {
	CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error)
	GetWalletByUserID(ctx context.Context, userID uint) (*dto.WalletResponse, error)
	TopUp(ctx context.Context, userID uint, request dto.TopUpRequest, idemKey string) (*dto.TopUpResponse, error)
}

type walletUsecaseImpl struct {
	walletRepo      repository.WalletRepository
	transactionRepo repository.TransactionRepository
	idemService     IdempotencyService
	log             *logrus.Logger
}

func NewWalletUsecase(walletRepo repository.WalletRepository, transactionRepo repository.TransactionRepository, idemService IdempotencyService, log *logrus.Logger) WalletUsecase {
	return &walletUsecaseImpl{walletRepo: walletRepo, transactionRepo: transactionRepo, idemService: idemService, log: log}
}

func (u *walletUsecaseImpl) CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error) {
	logger := u.log.WithFields(logrus.Fields{"user_id": userID})
	logger.Debug("attempting to create wallet")

	wallet := &entity.Wallet{
		UserID: userID,
	}

	err := u.walletRepo.Create(ctx, wallet)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			return nil, apperror.ErrUserHasWalletAlready
		}
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	logger.Info("wallet created successfully")
	return wallet, nil
}

func (u *walletUsecaseImpl) GetWalletByUserID(ctx context.Context, userID uint) (*dto.WalletResponse, error) {
	logger := u.log.WithFields(logrus.Fields{"user_id": userID})
	logger.Debug("attempting to get wallet")

	wallet, err := u.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			newWallet := &entity.Wallet{
				UserID:  userID,
				Balance: 0,
			}
			err = u.walletRepo.Create(ctx, newWallet)
			if err != nil {
				if errors.Is(err, apperror.ErrDuplicatedKey) {
					return nil, apperror.ErrUserHasWalletAlready
				}
				return nil, fmt.Errorf("create wallet: %w", err)
			}
			logger.Info("wallet created successfully")
			return &dto.WalletResponse{
				ID:        newWallet.ID,
				UserID:    newWallet.UserID,
				Balance:   newWallet.Balance,
				CreatedAt: newWallet.CreatedAt,
				UpdatedAt: newWallet.UpdatedAt,
			}, nil
		}
		return nil, fmt.Errorf("get wallet by user id %d: %w", userID, err)
	}

	logger.Info("wallet found successfully")
	return &dto.WalletResponse{
		ID:        wallet.ID,
		UserID:    wallet.UserID,
		Balance:   wallet.Balance,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}, nil
}

func (u *walletUsecaseImpl) TopUp(ctx context.Context, userID uint, request dto.TopUpRequest, idemKey string) (*dto.TopUpResponse, error) {
	logger := u.log.WithFields(logrus.Fields{
		"userID":  userID,
		"idemKey": idemKey,
		"amount":  request.Amount,
	})
	logger.Debug("attempting top up")

	if request.Amount <= 0 {
		return nil, apperror.ErrInvalidAmount
	}

	wallet, err := u.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrWalletNotFound
		}
		return nil, fmt.Errorf("find wallet by user id %d: %w", userID, err)
	}

	claimed, cachedBody, err := u.idemService.Claim(ctx, idemKey, userID, "TOPUP", request)
	if err != nil {
		return nil, err
	}

	if !claimed {
		var cached dto.TopUpResponse
		if err := json.Unmarshal([]byte(cachedBody), &cached); err != nil {
			return nil, fmt.Errorf("unmarshal cached top up response: %w", err)
		}
		logger.Info("top up wallet: duplicate request detected, returning cached response")
		return &cached, nil
	}

	var result *dto.TopUpResponse
	txErr := u.walletRepo.WithTx(ctx, func(tx *gorm.DB) error {
		lockWallet, err := u.walletRepo.LockByID(tx, wallet.ID)
		if err != nil {
			if errors.Is(err, apperror.ErrRecordNotFound) {
				return apperror.ErrWalletNotFound
			}
			return fmt.Errorf("lock wallet: %w", err)
		}

		balanceBefore := lockWallet.Balance

		lockWallet.Balance += request.Amount

		err = u.walletRepo.Save(tx, lockWallet)
		if err != nil {
			return fmt.Errorf("save wallet: %w", err)
		}

		transaction := &entity.Transaction{
			WalletID:      lockWallet.ID,
			Type:          entity.TxTypeTopup,
			Amount:        request.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  lockWallet.Balance,
			Status:        entity.TxStatusSuccess,
		}

		err = u.transactionRepo.Create(tx, transaction)
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		result = &dto.TopUpResponse{
			TransactionID: transaction.ID,
			WalletID:      transaction.WalletID,
			Type:          string(transaction.Type),
			Amount:        transaction.Amount,
			BalanceBefore: transaction.BalanceBefore,
			BalanceAfter:  transaction.BalanceAfter,
			Status:        string(transaction.Status),
			CreatedAt:     transaction.CreatedAt,
		}

		return nil
	})

	if txErr != nil {
		err := u.idemService.MarkFailed(ctx, idemKey)
		if err != nil {
			logger.WithError(err).Error("failed to mark idempotency key as failed")
		}
		return nil, txErr
	}

	err = u.idemService.Complete(ctx, idemKey, result)
	if err != nil {
		logger.WithError(err).Error("failed to mark idempotency key as complete")
	}

	logger.Info("top up wallet: completed successfully")
	return result, nil
}
