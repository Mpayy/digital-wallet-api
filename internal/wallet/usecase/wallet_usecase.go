package usecase

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/sirupsen/logrus"
)

type WalletUsecase interface {
	CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error)
	GetWalletByUserID(ctx context.Context, userID uint) (*dto.WalletResponse, error)
	TopUp(ctx context.Context, userID uint, req dto.TopUpRequest, idemKey string) (*dto.TopUpResponse, error)
}

type walletUsecaseImpl struct {
	walletRepo repository.WalletRepository
	log        *logrus.Logger
}

func NewWalletUsecase(walletRepo repository.WalletRepository, log *logrus.Logger) WalletUsecase {
	return &walletUsecaseImpl{walletRepo: walletRepo, log: log}
}

func (u *walletUsecaseImpl) CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error) {
	u.log.WithField("user_id", userID).Debug("Attempting to create wallet")

	wallet := &entity.Wallet{
		UserID: userID,
	}

	err := u.walletRepo.Create(ctx, wallet)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicatedKey) {
			u.log.WithFields(logrus.Fields{"user_id": userID, "error": err}).Warn("Failed to create wallet")
			return nil, apperror.ErrUserHasWalletAlready
		}
		u.log.WithFields(logrus.Fields{"user_id": userID, "error": err}).Error("Failed to create wallet")
		return nil, apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"user_id": userID}).Info("Wallet created successfully")
	return wallet, nil
}

func (u *walletUsecaseImpl) GetWalletByUserID(ctx context.Context, userID uint) (*dto.WalletResponse, error) {
	u.log.WithField("user_id", userID).Debug("Attempting to get wallet")

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
				return nil, apperror.ErrInternalServer
			}
			return &dto.WalletResponse{
				ID:        newWallet.ID,
				UserID:    newWallet.UserID,
				Balance:   newWallet.Balance,
				CreatedAt: newWallet.CreatedAt,
				UpdatedAt: newWallet.UpdatedAt,
			}, nil
		}
		return nil, err
	}

	return &dto.WalletResponse{
		ID:        wallet.ID,
		UserID:    wallet.UserID,
		Balance:   wallet.Balance,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}, nil
}

func (u *walletUsecaseImpl) TopUp(ctx context.Context, userID uint, req dto.TopUpRequest, idemKey string) (*dto.TopUpResponse, error) {
	// TODO: implementasi top up
	return &dto.TopUpResponse{}, nil
}
