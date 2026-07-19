package entity

import "time"

type Wallet struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	UserID    uint      `gorm:"column:user_id;uniqueIndex;not null"`// Sengaja tanpa FK ke users.id — Auth adalah bounded context terpisah (loosely coupled)
	Balance   int64     `gorm:"column:balance;not null;default:0"`// smallest unit; IDR gaada subunit, jadi ini langsung Rupiah
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Wallet) TableName() string { return "wallets" }
