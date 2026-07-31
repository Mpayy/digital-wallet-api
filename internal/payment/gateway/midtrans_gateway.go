package gateway

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Mpayy/digital-wallet-api/internal/payment/dto"
	"github.com/Mpayy/digital-wallet-api/internal/payment/entity"
	"github.com/Mpayy/digital-wallet-api/internal/pkg/apperror"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/spf13/viper"
)

type midtransGateway struct {
	snapClient *snap.Client
	serverKey  string
}

func NewMidtransGateway(config *viper.Viper) PaymentCollector {
	environment := midtrans.Sandbox
	if config.GetString("MIDTRANS_ENV") == "production" {
		environment = midtrans.Production
	}

	serverKey := config.GetString("MIDTRANS_SERVER_KEY")

	var snapClient snap.Client
	snapClient.New(serverKey, environment)

	return &midtransGateway{serverKey: serverKey, snapClient: &snapClient}
}

func (m *midtransGateway) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	snapReq := snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.Amount,
		},
	}

	redirectURL, err := m.snapClient.CreateTransactionUrl(&snapReq)
	if err != nil {
		return nil, err
	}

	return &ChargeResult{
		ProviderRefID: req.OrderID,
		RedirectURL:   redirectURL,
	}, nil
}

func (m *midtransGateway) VerifyAndParseWebhook(payload []byte) (*WebhookEvent, error) {
	var notif dto.MidtransWebhookPayload
	err := json.Unmarshal(payload, &notif)
	if err != nil {
		return nil, fmt.Errorf("unmarshal webhook payload: %w", err)
	}

	// Rumus Resmi Midtrans: SHA512(order_id + status_code + gross_amount + ServerKey)
	rawInput := notif.OrderID + notif.StatusCode + notif.GrossAmount + m.serverKey

	hash := sha512.New()
	hash.Write([]byte(rawInput))
	generatedSignature := hex.EncodeToString(hash.Sum(nil))

	if generatedSignature != notif.SignatureKey {
		return nil, apperror.ErrInvalidWebhookSignature
	}

	var normalizedStatus string
	switch notif.TransactionStatus {
	case "capture":
		if notif.FraudStatus == "accept" {
			normalizedStatus = string(entity.PaymentTransactionStatusSettled)
		} else {
			normalizedStatus = string(entity.PaymentTransactionStatusFailed)
		}
	case "settlement":
		normalizedStatus = string(entity.PaymentTransactionStatusSettled)
	case "pending":
		normalizedStatus = string(entity.PaymentTransactionStatusPending)
	case "deny", "cancel":
		normalizedStatus = string(entity.PaymentTransactionStatusFailed)
	case "expire":
		normalizedStatus = string(entity.PaymentTransactionStatusExpired)
	default:
		return nil, fmt.Errorf("unknown transaction status: %s", notif.TransactionStatus)
	}

	return &WebhookEvent{
		ProviderRefID: notif.OrderID,
		Status:        normalizedStatus,
	}, nil
}
