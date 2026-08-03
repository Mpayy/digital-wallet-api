package gateway

import (
	"context"
	"net/http"
)

const (
	MidtransTestNotifPrefix1 = "payment_notif_test_"
	MidtransTestNotifPrefix2 = "sample-"
)

type ChargeRequest struct {
	OrderID string
	Amount  int64
}

type ChargeResult struct {
	ProviderRefID string
	RedirectURL   string
}

type WebhookEvent struct {
	ProviderRefID string
	Status        string
}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_payment_collector.go
type PaymentCollector interface {
	CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
	VerifyAndParseWebhook(payload []byte) (*WebhookEvent, error)
}

type PayoutRequest struct {
	ReferenceID       string
	Amount            int64
	ChannelCode       string
	AccountNumber     string
	AccountHolderName string
}
type PayoutResult struct {
	ProviderRefID string
	Status        string // ACCEPTED — belum final
}
type PayoutWebhookEvent struct {
	ProviderRefID string
	Status        string // dinormalisasi: SUCCEEDED | FAILED
}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_payment_disburser.go
type PaymentDisburser interface {
	CreatePayout(ctx context.Context, req PayoutRequest) (*PayoutResult, error)
	VerifyWebhookSignature(headers http.Header) error
	ParsePayoutWebhook(payload []byte) (*PayoutWebhookEvent, error)
}
