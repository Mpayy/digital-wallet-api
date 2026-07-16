package dto

import "time"

type WalletResponse struct {
	ID uint `json:"id"`
	UserID uint `json:"user_id"`
	Balance int64 `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
