package gateway

import "context"

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

type PaymentCollector interface {
	CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
	VerifyWebhookSignature(payload []byte) error
	ParseWebhookPayload(payload []byte) (*WebhookEvent, error)
}
