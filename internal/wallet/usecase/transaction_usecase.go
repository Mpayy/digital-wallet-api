package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/sirupsen/logrus"
)

type TransactionUsecase interface {
	GetTransactionHistory(ctx context.Context, userID uint, filter dto.TransactionFilter) (*dto.TransactionListResponse, error)
	GetTransactionDetail(ctx context.Context, userID uint, transactionID uint) (*dto.TransactionResponse, error)
}

type transactionUsecaseImpl struct {
	transactionRepo repository.TransactionRepository
	walletUsecase   WalletUsecase
	log             *logrus.Logger
}

func NewTransactionUsecase(transactionRepo repository.TransactionRepository, walletUsecase WalletUsecase, log *logrus.Logger) TransactionUsecase {
	return &transactionUsecaseImpl{transactionRepo: transactionRepo, walletUsecase: walletUsecase, log: log}
}

func (u *transactionUsecaseImpl) GetTransactionHistory(ctx context.Context, userID uint, filter dto.TransactionFilter) (*dto.TransactionListResponse, error) {
	logger := u.log.WithFields(logrus.Fields{
		"user_id": userID,
		"filter":  filter,
	})
	logger.Debug("attempting to get transaction history")

	if filter.StartDate != "" && filter.EndDate == "" {
		filter.EndDate = time.Now().Format("2006-01-02")
	}

	if filter.Page == 0 {
		filter.Page = 1
	}

	if filter.Limit == 0 {
		filter.Limit = 10
	}

	wallet, err := u.walletUsecase.GetWalletByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	transactions, total, err := u.transactionRepo.FindByWalletID(ctx, wallet.ID, filter)
	if err != nil {
		return nil, fmt.Errorf("find transactions: %w", err)
	}

	txResponse := []dto.TransactionResponse{}
	for _, tx := range transactions {
		txResponse = append(txResponse, dto.TransactionResponse{
			TransactionID: tx.ID,
			Type:          string(tx.Type),
			Amount:        tx.Amount,
			BalanceBefore: tx.BalanceBefore,
			BalanceAfter:  tx.BalanceAfter,
			Status:        string(tx.Status),
			CreatedAt:     tx.CreatedAt,
		})
	}

	txListResponse := dto.TransactionListResponse{
		Data: txResponse,
		Meta: dto.MetaPagination{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: (total + int64(filter.Limit) - 1) / int64(filter.Limit),
		},
	}

	logger.Info("transaction history fetched successfully")
	return &txListResponse, nil
}

func (u *transactionUsecaseImpl) GetTransactionDetail(ctx context.Context, userID uint, transactionID uint) (*dto.TransactionResponse, error) {
	logger := u.log.WithFields(logrus.Fields{
		"user_id":        userID,
		"transaction_id": transactionID,
	})
	logger.Debug("attempting to get transaction detail")

	wallet, err := u.walletUsecase.GetWalletByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	transaction, err := u.transactionRepo.FindByID(ctx, transactionID)
	if err != nil {
		if errors.Is(err, apperror.ErrRecordNotFound) {
			return nil, apperror.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("find transaction: %w", err)
	}

	if transaction.WalletID != wallet.ID {
		logger.WithFields(logrus.Fields{
			"transaction_wallet_id": transaction.WalletID,
			"wallet_id":             wallet.ID,
		}).Warn("transaction not found")
		return nil, apperror.ErrTransactionNotFound
	}

	txResponse := dto.TransactionResponse{
		TransactionID: transaction.ID,
		Type:          string(transaction.Type),
		Amount:        transaction.Amount,
		BalanceBefore: transaction.BalanceBefore,
		BalanceAfter:  transaction.BalanceAfter,
		Status:        string(transaction.Status),
		CreatedAt:     transaction.CreatedAt,
	}

	return &txResponse, nil
}
