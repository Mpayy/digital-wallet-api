//go:build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentTopUp_SameIdempotencyKey_OnlyAppliedOnce(t *testing.T) {
	db := setupIntegrationDB(t)
	walletUC := setupWalletUsecase(t, db)

	wallet := seedWallet(t, db, 1, 0)
	idemKey := uuid.NewString() // SAMA buat semua goroutine — inilah yang mau dites

	const goroutines = 20
	var wg sync.WaitGroup
	results := make([]*dto.TopUpResponse, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := walletUC.TopUp(context.Background(), wallet.UserID,
				dto.TopUpRequest{Amount: 1000}, idemKey)
			results[i] = resp
			errs[i] = err
		}(i)
	}
	wg.Wait()

	var succeeded []*dto.TopUpResponse
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded = append(succeeded, results[i])
		case errors.Is(err, apperror.ErrRequestInProgress):
			// EXPECTED — goroutine ini kalah race, ngecek pas pemenang belum commit.
			// Bukti mekanisme claim nolak concurrent duplicate dengan benar, bukan kegagalan.
		default:
			t.Errorf("goroutine %d: error tidak terduga: %v", i, err)
		}
	}

	require.NotEmpty(t, succeeded, "minimal satu goroutine harus berhasil (asli atau replay)")

	firstTxID := succeeded[0].TransactionID
	for _, r := range succeeded {
		assert.Equal(t, firstTxID, r.TransactionID, "semua response sukses harus merujuk transaksi yang SAMA")
	}

	var final entity.Wallet
	require.NoError(t, db.First(&final, wallet.ID).Error)
	assert.Equal(t, int64(1000), final.Balance, "saldo cuma boleh naik SEKALI walau di-fire 20x bersamaan")

	var txCount int64
	db.Model(&entity.Transaction{}).Where("wallet_id = ?", wallet.ID).Count(&txCount)
	assert.EqualValues(t, 1, txCount, "cuma boleh ada SATU baris transaksi")
}
