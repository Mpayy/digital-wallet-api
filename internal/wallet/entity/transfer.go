package entity

import "time"

type Transfer struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	FromWalletID uint      `gorm:"not null;index" json:"from_wallet_id"`
	ToWalletID   uint      `gorm:"not null;index" json:"to_wallet_id"`
	Amount       int64     `gorm:"not null" json:"amount"`
	Note         string    `gorm:"type:varchar(255)" json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (Transfer) TableName() string { return "transfers" }
