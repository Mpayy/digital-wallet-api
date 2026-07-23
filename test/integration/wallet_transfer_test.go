//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/wallet/dto"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



func TestConcurrentTransfer_OppositeDirection_NoDeadlock(t *testing.T) {
	db := setupIntegrationDB(t)
	transferUC := setupTransferUsecase(t, db) // pola sama kayak setupWalletUsecase

	walletA := seedWallet(t, db, 1, 1_000_000)
	walletB := seedWallet(t, db, 2, 1_000_000)

	const iterations = 50
	var wg sync.WaitGroup
	errCh := make(chan error, iterations*2)

	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := transferUC.Transfer(context.Background(), walletA.UserID,
				dto.TransferRequest{ToUserID: walletB.UserID, Amount: 1000}, uuid.NewString())
			if err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			_, err := transferUC.Transfer(context.Background(), walletB.UserID,
				dto.TransferRequest{ToUserID: walletA.UserID, Amount: 1000}, uuid.NewString())
			if err != nil {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		// semua goroutine selesai normal
	case <-time.After(15 * time.Second):
		t.Fatal("test timeout — kemungkinan deadlock, transaction ada yang stuck nunggu selamanya")
	}

	close(errCh)
	for err := range errCh {
		t.Errorf("unexpected transfer error: %v", err)
	}

	var finalA, finalB entity.Wallet
	require.NoError(t, db.First(&finalA, walletA.ID).Error)
	require.NoError(t, db.First(&finalB, walletB.ID).Error)

	assert.Equal(t, int64(1_000_000), finalA.Balance)
	assert.Equal(t, int64(1_000_000), finalB.Balance)
	assert.Equal(t, int64(2_000_000), finalA.Balance+finalB.Balance) // konservasi total uang

	var txCount int64
	db.Model(&entity.Transaction{}).Count(&txCount)
	assert.EqualValues(t, iterations*4, txCount) // tiap transfer = 2 baris (OUT+IN) x 2 arah x iterations
}
