package entity

import "time"

type Wallet struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    uint      `gorm:"column:user_id;not null;unique"`
	Balance   int64     `gorm:"column:balance;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Wallet) TableName() string { return "wallets" }
