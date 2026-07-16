package dto

import "github.com/Mpayy/digital-wallet-api/internal/wallet/entity"

type TransactionFilter struct {
	Type      entity.TransactionType `form:"type" validate:"omitempty,oneof=TOPUP TRANSFER_IN TRANSFER_OUT"`
	StartDate string                 `form:"start_date" validate:"omitempty,datetime=2006-01-02,ltefield=EndDate"`
	EndDate   string                 `form:"end_date" validate:"omitempty,datetime=2006-01-02"`
	Page      int                    `form:"page" validate:"omitempty,gte=1"`
	Limit     int                    `form:"limit" validate:"omitempty,gte=1,lte=100"`
}
