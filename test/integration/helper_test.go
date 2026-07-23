//go:build integration

package integration_test

import (
	"io"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/repository"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/usecase"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/sirupsen/logrus"
)

func setupIntegrationDB(t *testing.T) *gorm.DB {
	dsn := "root@tcp(localhost:3307)/digital_wallet_test?parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("SET FOREIGN_KEY_CHECKS = 0")
		for _, table := range []string{"transactions", "transfers", "wallets", "idempotency_keys"} {
			db.Exec("TRUNCATE TABLE " + table)
		}
		db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	})

	return db
}

func seedWallet(t *testing.T, db *gorm.DB, userID uint, balance int64) *entity.Wallet {
	w := &entity.Wallet{UserID: userID, Balance: balance}
	require.NoError(t, db.Create(w).Error)
	return w
}

// Wiring manual — bukan lewat Wire/wire_gen.go, karena di sini kamu cuma butuh
// usecase+repo+db, bukan seluruh app (router, middleware, dst)
func setupWalletUsecase(t *testing.T, db *gorm.DB) usecase.WalletUsecase {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db)
	idemService := usecase.NewIdempotencyService(logger, idemRepo)

	return usecase.NewWalletUsecase(walletRepo, transactionRepo, idemService, logger)
}

func setupTransferUsecase(t *testing.T, db *gorm.DB) usecase.TransferUsecase {
	// logic sama persis kayak setupWalletUsecase, cuma ganti nama return entity
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	walletRepo := repository.NewWalletRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db)
	idemService := usecase.NewIdempotencyService(logger, idemRepo)

	return usecase.NewTransferUsecase(transferRepo, walletRepo, idemService, transactionRepo, logger)
}