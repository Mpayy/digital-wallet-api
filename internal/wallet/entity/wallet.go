package entity

import "time"

type Wallet struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	UserID    uint      `gorm:"column:user_id;uniqueIndex;not null"`
	Balance   int64     `gorm:"column:balance;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Wallet) TableName() string { return "wallets" }
