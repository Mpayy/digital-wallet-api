package dto

import "time"

type TopUpRequest struct {
	Amount int64 `json:"amount" validate:"required,gt=0"`
}

type TopUpResponse struct {
	TransactionID uint      `json:"transaction_id"`
	WalletID      uint      `json:"wallet_id"`
	Type          string    `json:"type"`
	Amount        int64     `json:"amount"`
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
