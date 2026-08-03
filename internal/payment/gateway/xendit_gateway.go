package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/spf13/viper"
	"github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/payout"
)

type xenditGateway struct {
	xenditClient *xendit.APIClient
	webhookToken string
}

func NewXenditGateway(config *viper.Viper) PaymentDisburser {
	secretKey := config.GetString("XENDIT_SECRET_KEY")
	webhookToken := config.GetString("XENDIT_WEBHOOK_TOKEN")

	xenditClient := xendit.NewClient(secretKey)

	return &xenditGateway{
		xenditClient: xenditClient,
		webhookToken: webhookToken,
	}
}

func (x *xenditGateway) CreatePayout(ctx context.Context, req PayoutRequest) (*PayoutResult, error) {
	const maxSafeFloat32Amount = 1 << 24 // 16.777.216 — di atas ini float32 nggak exact lagi
	if req.Amount > maxSafeFloat32Amount {
		return nil, fmt.Errorf("amount %d exceeds safe precision for payout gateway", req.Amount)
	}

	channelProps := payout.NewDigitalPayoutChannelProperties(req.AccountNumber)
	channelProps.SetAccountHolderName(req.AccountHolderName)

	payoutReq := *payout.NewCreatePayoutRequest(req.ReferenceID, req.ChannelCode, *channelProps, float32(req.Amount), "IDR")

	resp, _, err := x.xenditClient.PayoutApi.CreatePayout(ctx).
		IdempotencyKey(req.ReferenceID). // key SAMA yang dipake buat reference_id — deterministik per withdrawal
		CreatePayoutRequest(payoutReq).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}

	return &PayoutResult{
		ProviderRefID: resp.Payout.ReferenceId,
		Status:        resp.Payout.Status,
	}, nil
}

func (x *xenditGateway) VerifyWebhookSignature(headers http.Header) error {
	token := headers.Get("x-callback-token")
	if token == "" || token != x.webhookToken {
		return apperror.ErrInvalidWebhookSignature
	}
	return nil
}

func (x *xenditGateway) ParsePayoutWebhook(payload []byte) (*PayoutWebhookEvent, error) {
	var notif dto.XenditPayoutWebhookPayload
	if err := json.Unmarshal(payload, &notif); err != nil {
		return nil, fmt.Errorf("unmarshal payout webhook payload: %w", err)
	}

	var normalizedStatus string
	switch notif.Data.Status {
	case "SUCCEEDED":
		normalizedStatus = "SUCCEEDED"
	case "FAILED", "CANCELLED", "REVERSED":
		normalizedStatus = "FAILED"
	case "ACCEPTED", "REQUESTED":
		// status TRANSISI, belum final — jangan panggil Finalize/Reverse buat ini
		normalizedStatus = "PENDING"
	default:
		return nil, fmt.Errorf("unknown payout status: %s", notif.Data.Status)
	}

	return &PayoutWebhookEvent{
		ProviderRefID: notif.Data.ReferenceID,
		Status:        normalizedStatus,
	}, nil
}
