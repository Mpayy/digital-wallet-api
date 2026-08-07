//go:build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentCheckout_SameIdempotencyKey_OnlyOneChargeCreated(t *testing.T) {
	db := setupIntegrationDB(t)
	stubGW := &stubPaymentCollector{}
	ch := setupRabbitMQ(t)
	paymentUC := setupPaymentUsecase(t, db, stubGW, ch)

	idemKey := uuid.NewString()
	const goroutines = 15

	var wg sync.WaitGroup
	results := make(chan *dto.CheckoutResponse, goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := paymentUC.CreateTopUpCheckout(context.Background(), 1, 50_000, idemKey)
			if err != nil {
				errs <- err
				return
			}
			results <- resp
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if !errors.Is(err, apperror.ErrRequestInProgress) {
			t.Errorf("unexpected checkout error: %v", err)
		}
	}

	var urls []string
	for r := range results {
		urls = append(urls, r.RedirectURL)
	}
	require.NotEmpty(t, urls)
	for _, u := range urls {
		assert.Equal(t, urls[0], u, "semua response sukses harus ngarah ke checkout session yang SAMA")
	}

	assert.Equal(t, 1, stubGW.callCount, "Midtrans cuma boleh ke-hit SEKALI walau 15 request race bersamaan")
}
