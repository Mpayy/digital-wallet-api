package gateway

import "context"

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
