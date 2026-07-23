//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
)

func TestConcurrentTopUp_NoLostUpdate(t *testing.T) {
	db := setupIntegrationDB(t)
	walletUC := setupWalletUsecase(t, db)

	wallet := seedWallet(t, db, 1, 0)

	const goroutines = 20
	const amountEach = int64(1000)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// idempotency key BEDA tiap goroutine — ini genuinely N top-up terpisah, bukan retry
			_, err := walletUC.TopUp(context.Background(), wallet.UserID,
				dto.TopUpRequest{Amount: amountEach}, uuid.NewString())
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("unexpected topup error: %v", err)
	}

	var final entity.Wallet
	require.NoError(t, db.First(&final, wallet.ID).Error)
	assert.Equal(t, amountEach*goroutines, final.Balance) // <- INI yang buktiin row lock jalan

	var txCount int64
	db.Model(&entity.Transaction{}).Where("wallet_id = ?", wallet.ID).Count(&txCount)
	assert.EqualValues(t, goroutines, txCount) // buktiin nggak ada write yang ke-drop
}
