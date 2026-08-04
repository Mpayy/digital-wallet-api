//go:build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/Mpayy/digital-wallet-api/internal/wallet/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentWithdrawal_NoNegativeBalance(t *testing.T) {
	db := setupIntegrationDB(t)
	walletUC := setupWalletUsecase(t, db) // helper yang SAMA persis dari T19

	wallet := seedWallet(t, db, 1, 100_000)

	const goroutines = 20
	const amountEach = int64(10_000)
	// total permintaan = 200.000, saldo cuma 100.000 -> harus PERSIS 10 yang lolos, 10 ditolak

	var wg sync.WaitGroup
	results := make(chan error, goroutines) // buffered sebesar jumlah goroutine — biar goroutine yang kirim
	// hasil nggak perlu NUNGGU ada yang baca duluan

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := walletUC.Withdraw(context.Background(), wallet.UserID, amountEach, uuid.NewString())
			results <- err // kirim HASIL (nil kalau sukses, error kalau nggak) ke channel — apapun hasilnya, tetep dikirim
		}()
	}

	go func() {
		wg.Wait()      // tunggu SEMUA goroutine kelar ngirim ke channel
		close(results) // baru tutup channel — sinyal ke `range` di bawah "udah nggak ada lagi yang dateng"
	}()

	var succeeded, insufficientBalance, unexpected int
	for err := range results { // blocking-read: nunggu tiap item masuk, otomatis berhenti pas channel ditutup
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, apperror.ErrInsufficientBalance):
			insufficientBalance++
		default:
			unexpected++
			t.Errorf("unexpected withdrawal error: %v", err)
		}
	}

	assert.Equal(t, 0, unexpected)
	assert.Equal(t, 10, succeeded)           // 100.000 / 10.000 = TEPAT 10, bukan lebih bukan kurang
	assert.Equal(t, 10, insufficientBalance) // sisanya HARUS ditolak, bukan lolos dengan salah

	var final entity.Wallet
	require.NoError(t, db.First(&final, wallet.ID).Error)
	assert.Equal(t, int64(0), final.Balance)
	assert.True(t, final.Balance >= 0, "saldo TIDAK BOLEH pernah negatif — ini bukti utama row lock jalan")
}
