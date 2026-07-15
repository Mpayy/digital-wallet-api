package usecase

import (
	"context"
	"errors"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/sirupsen/logrus"
)

type WalletUsecase interface {
	CreateWallet(ctx context.Context, userID uint) (*entity.Wallet, error)
}

type walletUsecaseImpl struct {
	walletRepo repository.WalletRepository
	log *logrus.Logger
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
		if errors.Is(err, apperror.ErrDuplicatedKey){
			u.log.WithFields(logrus.Fields{"user_id": userID, "error": err}).Warn("Failed to create wallet")
			return nil, apperror.ErrUserHasWalletAlready
		}
		u.log.WithFields(logrus.Fields{"user_id": userID, "error": err}).Error("Failed to create wallet")
		return nil, apperror.ErrInternalServer
	}

	u.log.WithFields(logrus.Fields{"user_id": userID}).Info("Wallet created successfully")
	return wallet, nil
}
