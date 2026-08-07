package main

import (
	"context"

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
	return w.consumer.Consume(ctx, queue.MidtransWebhookQueue, func(body []byte) error {
		return w.paymentUsecase.HandleWebhook(ctx, body)
	})
}
