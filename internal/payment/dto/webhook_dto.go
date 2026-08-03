package dto

type MidtransWebhookPayload struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

// type XenditPayoutWebhookPayload struct {
// 	Data struct {
// 		ID           string `json:"id"`
// 		ReferenceID    string `json:"reference_id"`
// 		Amount       int64  `json:"amount"`
// 		Status       string `json:"status"`
// 		Currency     string `json:"currency"`
// 		BankCode     string `json:"bank_code"`
// 		AccountNumber  string `json:"account_number"`
// 		AccountHolderName string `json:"account_holder_name"`
// 		XenditFee      int64  `json:"xendit_fee"`
// 		EstimatedArrival string `json:"estimated_arrival"`
// 		RequestedAt    string `json:"requested_at"`
// 		SucceededAt  string `json:"succeeded_at"`
// 		FailedAt     string `json:"failed_at"`
// 		ReversedAt   string `json:"reversed_at"`
// 		CreatedBy      string `json:"created_by"`
// 	} `json:"data"`
// }

type XenditPayoutWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		ReferenceID string `json:"reference_id"`
		Status      string `json:"status"`
	} `json:"data"`
}