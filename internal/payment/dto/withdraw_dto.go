package dto

import "time"

type WithdrawalRequest struct {
	ChannelCode       string `json:"channel_code" validate:"required"`
	AccountNumber     string `json:"account_number" validate:"required"`
	AccountHolderName string `json:"account_holder_name" validate:"required"`
	Amount            int64  `json:"amount" validate:"required,gt=0"`
}

type WithdrawalResponse struct {
	TransactionID     uint      `json:"transaction_id"`
	WalletID          uint      `json:"wallet_id"`
	Amount            int64     `json:"amount"`
	BalanceBefore     int64     `json:"balance_before"`
	BalanceAfter      int64     `json:"balance_after"`
	ChannelCode       string    `json:"channel_code"` // ganti nama, alasan di bawah
	AccountNumber     string    `json:"account_number"`
	AccountHolderName string    `json:"account_holder_name"`
	ReferenceID       string    `json:"reference_id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}
