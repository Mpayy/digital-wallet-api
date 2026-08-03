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

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_wallet_usecase.go
type WalletUsecase interface {
	CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error)
	GetWalletByUserID(ctx context.Context, userID uint) (*dto.WalletResponse, error)
	TopUp(ctx context.Context, userID uint, req dto.TopUpRequest, idemKey string) (*dto.TopUpResponse, error)
	Withdraw(ctx context.Context, userID uint, amount int64, idemKey string) (*dto.WithdrawResponse, error)
	ReverseWithdrawal(ctx context.Context, transactionID uint, reason string) error
	FinalizeWithdrawal(ctx context.Context, transactionID uint) error
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

func (u *walletUsecaseImpl) Withdraw(ctx context.Context, userID uint, amount int64, idemKey string) (*dto.WithdrawResponse, error) {
	logger := u.log.WithFields(logrus.Fields{
		"user_id": userID,
		"amount":  amount,
		"idemKey": idemKey,
	})
	logger.Debug("attempting withdrawal")

	if amount <= 0 {
		return nil, apperror.ErrInvalidAmount
	}

	wallet, err := u.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrWalletNotFound
		}
		return nil, fmt.Errorf("find wallet: %w", err)
	}

	claimed, cachedBody, err := u.idemService.Claim(ctx, idemKey, userID, "WITHDRAWAL", dto.WithdrawClaimPayload{Amount: amount})
	if err != nil {
		return nil, err
	}
	if !claimed {
		var cached dto.WithdrawResponse
		if err := json.Unmarshal([]byte(cachedBody), &cached); err != nil {
			return nil, fmt.Errorf("unmarshal cached withdraw response: %w", err)
		}
		return &cached, nil
	}

	var result *dto.WithdrawResponse
	txErr := u.walletRepo.WithTx(ctx, func(tx *gorm.DB) error {
		locked, err := u.walletRepo.LockByID(tx, wallet.ID)
		if err != nil {
			if errors.Is(err, apperror.ErrRecordNotFound) {
				return apperror.ErrWalletNotFound
			}
			return fmt.Errorf("lock wallet: %w", err)
		}

		if locked.Balance < amount {
			return apperror.ErrInsufficientBalance
		}

		before := locked.Balance
		locked.Balance -= amount

		err = u.walletRepo.Save(tx, locked)
		if err != nil {
			return fmt.Errorf("save wallet: %w", err)
		}

		transaction := &entity.Transaction{
			WalletID:      locked.ID,
			Type:          entity.TxTypeWithdrawal,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  locked.Balance,
			Status:        entity.TxStatusPending, // <- BEDA dari TopUp/Transfer: bukan langsung SUCCESS
		}

		err = u.transactionRepo.Create(tx, transaction)
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		result = &dto.WithdrawResponse{
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
		u.idemService.MarkFailed(ctx, idemKey)
		return nil, txErr
	}

	err = u.idemService.Complete(ctx, idemKey, result)
	if err != nil {
		u.log.WithError(err).Error("withdrawal debited but failed to mark idempotency completed")
	}

	logger.WithFields(logrus.Fields{
		"transaction_id": result.TransactionID,
	}).Info("wallet debited for withdrawal, pending gateway confirmation")

	return result, nil
}

func (u *walletUsecaseImpl) ReverseWithdrawal(ctx context.Context, transactionID uint, reason string) error {
	logger := u.log.WithFields(logrus.Fields{
		"transaction_id": transactionID,
		"reason":         reason,
	})
	logger.Debug("attempting to reverse withdrawal")

	transaction, err := u.transactionRepo.FindByID(ctx, transactionID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return apperror.ErrTransactionNotFound
		}
		return fmt.Errorf("find transaction: %w", err)
	}

	if transaction.Type != entity.TxTypeWithdrawal {
		return apperror.ErrInvalidTransactionType
	}

	if transaction.Status != entity.TxStatusPending {
		return apperror.ErrTransactionAlreadyReversed
	}

	if transaction.Amount <= 0 {
		return apperror.ErrInvalidAmount
	}

	txErr := u.walletRepo.WithTx(ctx, func(tx *gorm.DB) error {
		err = u.transactionRepo.UpdateStatus(tx, transactionID, entity.TxStatusFailed)
		if err != nil {
			if errors.Is(err, apperror.ErrRecordNotFound) {
				return apperror.ErrTransactionAlreadyReversed
			}
			return fmt.Errorf("update transaction: %w", err)
		}

		locked, err := u.walletRepo.LockByID(tx, transaction.WalletID)
		if err != nil {
			if errors.Is(err, apperror.ErrRecordNotFound) {
				return apperror.ErrWalletNotFound
			}
			return fmt.Errorf("lock wallet: %w", err)
		}

		locked.Balance += transaction.Amount

		err = u.walletRepo.Save(tx, locked)
		if err != nil {
			return fmt.Errorf("save wallet: %w", err)
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	logger.Debug("withdrawal reversed successfully")
	return nil
}

func (u *walletUsecaseImpl) FinalizeWithdrawal(ctx context.Context, transactionID uint) error {
	logger := u.log.WithFields(logrus.Fields{
		"transaction_id": transactionID,
	})
	logger.Debug("attempting to finalize withdrawal")

	txErr := u.walletRepo.WithTx(ctx, func(tx *gorm.DB) error {
		err := u.transactionRepo.UpdateStatus(tx, transactionID, entity.TxStatusSuccess)
		if err != nil {
			if errors.Is(err, apperror.ErrRecordNotFound) {
				return apperror.ErrTransactionAlreadyReversed
			}
			return fmt.Errorf("update transaction: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	logger.Debug("withdrawal finalized successfully")
	return nil
}
