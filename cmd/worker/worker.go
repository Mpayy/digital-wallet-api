package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	paymentUsecase "github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
	"golang.org/x/sync/errgroup"
)

type Worker struct {
	consumer          queue.Consumer
	paymentUsecase    paymentUsecase.PaymentUsecase
	withdrawalUsecase paymentUsecase.WithdrawalUsecase
}

func NewWorker(consumer queue.Consumer, pu paymentUsecase.PaymentUsecase, wu paymentUsecase.WithdrawalUsecase) *Worker {
	return &Worker{consumer: consumer, paymentUsecase: pu, withdrawalUsecase: wu}
}

func (w *Worker) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	consumers := map[string]func(body []byte) error{
		queue.MidtransWebhookQueue: w.handleMidtransWebhook,
		queue.XenditWebhookQueue:   w.handleXenditWebhook,
	}

	for qn, h := range consumers {
		g.Go(func() error {
			if err := w.consumer.Consume(ctx, qn, h); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("consumer for %s stopped unexpectedly: %w", qn, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (w *Worker) handleMidtransWebhook(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := w.paymentUsecase.HandleWebhook(ctx, body); err != nil {
		return err
	}

	return nil
}

func (w *Worker) handleXenditWebhook(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := w.withdrawalUsecase.HandlePayoutWebhook(ctx, body); err != nil {
		return err
	}

	return nil
}
