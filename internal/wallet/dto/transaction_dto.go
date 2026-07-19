package dto

import "time"

type TransactionFilter struct {
	Type      string `form:"type" validate:"omitempty,oneof=TOPUP TRANSFER_IN TRANSFER_OUT"`
	StartDate string `form:"start_date" validate:"omitempty,datetime=2006-01-02,ltefield=EndDate"`
	EndDate   string `form:"end_date" validate:"omitempty,datetime=2006-01-02"`
	Page      int    `form:"page" validate:"omitempty,gte=1"`
	Limit     int    `form:"limit" validate:"omitempty,gte=1,lte=100"`
}

type TransactionResponse struct {
	TransactionID uint   `json:"transaction_id"`
	Type          string `json:"type"`
	Amount        int64   `json:"amount"`
	BalanceBefore int64   `json:"balance_before"`
	BalanceAfter  int64   `json:"balance_after"`
	Status        string `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
}

type TransactionListResponse struct {
	Data []TransactionResponse `json:"data"`
	Meta MetaPagination        `json:"meta"`
}

type MetaPagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}
