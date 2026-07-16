package entity

import "time"

type Transfer struct {
	ID           uint      `gorm:"column:id;primaryKey" json:"id"`
	FromWalletID uint      `gorm:"column:from_wallet_id;not null;index" json:"from_wallet_id"`
	ToWalletID   uint      `gorm:"column:to_wallet_id;not null;index" json:"to_wallet_id"`
	Amount       int64     `gorm:"column:amount;not null" json:"amount"`
	Note         string    `gorm:"column:note;type:varchar(255)" json:"note,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Transfer) TableName() string { return "transfers" }
