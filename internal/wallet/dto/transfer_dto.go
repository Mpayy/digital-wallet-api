package dto

import "time"

type TransferResponse struct {
	TransferID         uint   `json:"transfer_id"`
	TransactionID      uint   `json:"transaction_id"`
	Type               string `json:"type"`
	Amount             int64  `json:"amount"`
	BalanceBefore      int64  `json:"balance_before"`
	BalanceAfter       int64  `json:"balance_after"`
	CounterPartyUserID int    `json:"counterparty_user_id"`
	Status             string `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}
