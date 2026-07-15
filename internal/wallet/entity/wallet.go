package entity

import "time"

type Wallet struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    UserID    uint      `gorm:"uniqueIndex;not null" json:"user_id"` // NO FK ke users — Auth = bounded context lain
    Balance   int64     `gorm:"not null;default:0" json:"balance"`  // smallest unit; IDR gaada subunit, jadi ini langsung Rupiah
    Version   uint      `gorm:"not null;default:0" json:"-"`        // optimistic-lock backstop, BUKAN mekanisme utama (lihat Bagian 3)
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Wallet) TableName() string { return "wallets" }