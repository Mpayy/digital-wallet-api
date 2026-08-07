package main

import (
	"context"
	"time"

	paymentUsecase "github.com/Mpayy/digital-wallet-api/internal/payment/usecase"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
)

type Worker struct {
	consumer       queue.Consumer
	paymentUsecase paymentUsecase.PaymentUsecase
}

func NewWorker(consumer queue.Consumer, pu paymentUsecase.PaymentUsecase) *Worker {
	return &Worker{consumer: consumer, paymentUsecase: pu}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.consumer.Consume(ctx, queue.MidtransWebhookQueue, w.handleMidtransWebhook); err != nil {
		return err
	}
	return nil
}

func (w *Worker) handleMidtransWebhook(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := w.paymentUsecase.HandleWebhook(ctx, body); err != nil {
		return err
	}

	return nil
}
