package entity

import "time"

type Transfer struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	FromWalletID uint      `gorm:"column:from_wallet_id;not null;index"`
	ToWalletID   uint      `gorm:"column:to_wallet_id;not null;index"`
	Amount       int64     `gorm:"column:amount;not null"`
	Note         string    `gorm:"column:note;type:varchar(255)"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (Transfer) TableName() string { return "transfers" }
