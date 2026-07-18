package dto

import "time"

type TransferRequest struct {
	ToUserID uint   `json:"to_user_id" validate:"required"`
	Amount   int64  `json:"amount" validate:"required,gt=0"`
	Note     string `json:"note" validate:"omitempty,max=255"`
}

type TransferResponse struct {
	TransferID         uint      `json:"transfer_id"`
	TransactionID      uint      `json:"transaction_id"`
	Type               string    `json:"type"`
	Amount             int64     `json:"amount"`
	BalanceBefore      int64     `json:"balance_before"`
	BalanceAfter       int64     `json:"balance_after"`
	CounterPartyUserID uint      `json:"counterparty_user_id"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}
